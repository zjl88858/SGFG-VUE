package filterhelper

import (
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"sgfg-ng/internal/sockfilter"
)

type fakeSession struct {
	closed atomic.Int32
	err    error
}

func (s *fakeSession) Close() error { s.closed.Add(1); return s.err }

func TestHelperLifecycleAndParentBypass(t *testing.T) {
	for _, failCleanup := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "cleanup-error"}[failCleanup], func(t *testing.T) {
			gui, helper := net.Pipe()
			s := &fakeSession{}
			if failCleanup {
				s.err = errors.New("driver busy")
			}
			serverDone := make(chan error, 1)
			go func() {
				serverDone <- serve(helper, 1234, func(port int, names []string, log func(string, string), opts sockfilter.Options) (filterSession, error) {
					if port != 7788 || len(names) != 1 || names[0] != "curl.exe" || !opts.TCPOnly || opts.BypassPID != 1234 {
						t.Error("incorrect fixed startup parameters")
					}
					log("info", "test startup")
					return s, nil
				})
			}()
			c, err := startClient(gui, message{Type: "start", Port: 7788, Names: []string{"curl.exe"}, TCPOnly: true}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := c.w.send(message{Type: "ping"}); err != nil {
				t.Fatal(err)
			}
			err = c.Close()
			if (err != nil) != failCleanup {
				t.Fatalf("cleanup result: %v", err)
			}
			if failCleanup && !strings.Contains(err.Error(), "driver busy") {
				t.Fatal(err)
			}
			_ = c.Close()
			select {
			case <-serverDone:
			case <-time.After(2 * time.Second):
				t.Fatal("helper did not exit")
			}
			if s.closed.Load() != 1 {
				t.Fatal("driver cleanup not exactly once")
			}
		})
	}
}

func TestHelperCleansOnBrokenPipe(t *testing.T) {
	gui, helper := net.Pipe()
	s := &fakeSession{}
	done := make(chan error, 1)
	go func() {
		done <- serve(helper, 123, func(int, []string, func(string, string), sockfilter.Options) (filterSession, error) { return s, nil })
	}()
	c, err := startClient(gui, message{Type: "start", Port: 7788, Names: []string{"curl.exe"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	gui.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pipe loss did not trigger cleanup")
	}
	<-c.Done()
	if s.closed.Load() != 1 || c.Err() == nil {
		t.Fatal("broken pipe not reported/cleaned")
	}
}

func TestHelperStartupFailure(t *testing.T) {
	gui, helper := net.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- serve(helper, 123, func(int, []string, func(string, string), sockfilter.Options) (filterSession, error) {
			return nil, errors.New("signature rejected")
		})
	}()
	_, err := startClient(gui, message{Type: "start", Port: 7788, Names: []string{"curl.exe"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "signature rejected") {
		t.Fatal(err)
	}
	<-done
}

func TestParentDiesDuringDriverStartup(t *testing.T) {
	gui, helper := net.Pipe()
	s := &fakeSession{}
	starting, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- serve(helper, 123, func(int, []string, func(string, string), sockfilter.Options) (filterSession, error) {
			close(starting)
			<-release
			return s, nil
		})
	}()
	w := &wire{connection: gui}
	if err := w.send(message{Type: "start", Port: 7788, Names: []string{"curl.exe"}}); err != nil {
		t.Fatal(err)
	}
	<-starting
	gui.Close()
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("startup orphan not cleaned")
	}
	if s.closed.Load() != 1 {
		t.Fatal("successful SDK init after parent death leaked session")
	}
}

func TestProtocolRejectsUnboundedUnknownAndWrongVersion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		size    uint32
		payload string
	}{
		{"oversize", maxMessage + 1, ""},
		{"empty", 0, ""},
		{"unknown-field", 0, `{"version":1,"type":"start","command":"evil"}`},
		{"wrong-version", 0, `{"version":2,"type":"stop"}`},
		{"multiple-json", 0, `{"version":1,"type":"stop"} {}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			go func() {
				defer b.Close()
				size := tc.size
				if tc.payload != "" {
					size = uint32(len(tc.payload))
				}
				var h [4]byte
				binary.LittleEndian.PutUint32(h[:], size)
				_ = writeAll(b, append(h[:], []byte(tc.payload)...))
			}()
			if _, err := (&wire{connection: a}).receive(time.Second); err == nil {
				t.Fatal("invalid message accepted")
			}
		})
	}
}

func TestInvalidStartDoesNotTouchDriver(t *testing.T) {
	for _, m := range []message{
		{Type: "start", Port: 7788},
		{Type: "start", Port: 0, Names: []string{"curl.exe"}},
		{Type: "start", Port: 7788, Names: []string{`C:\evil.exe`}},
		{Type: "install", Port: 7788, Names: []string{"curl.exe"}},
	} {
		gui, helper := net.Pipe()
		done := make(chan error, 1)
		go func() {
			done <- serve(helper, 123, func(int, []string, func(string, string), sockfilter.Options) (filterSession, error) {
				t.Error("invalid request touched driver")
				return &fakeSession{}, nil
			})
		}()
		w := &wire{connection: gui}
		if err := w.send(m); err != nil {
			t.Fatal(err)
		}
		response, err := w.receive(time.Second)
		if err != nil || response.Type != "error" {
			t.Fatal(response, err)
		}
		gui.Close()
		<-done
	}
}
