package core

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	relay_protocol "github.com/go-gost/relay"
	"github.com/go-gost/x/registry"
	xproxy "golang.org/x/net/proxy"
)

// Local-only integration: real SOCKS and HTTP handlers, real relay negotiation,
// and a fake upstream. No driver, credentials or Internet access required.
// The unmetered local forwarding stage exercises the same accounting boundary
// used by the SS relay, without a process-global SS salt replay-cache collision.
func TestGostTrafficAndLocalRelayCountOnce(t *testing.T) {
	upstream, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	request := []byte("upload-payload")
	response := bytes.Repeat([]byte("download"), 32)
	go func() {
		for {
			c, err := upstream.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				c.SetDeadline(time.Now().Add(3 * time.Second))
				var req relay_protocol.Request
				if _, err := req.ReadFrom(c); err != nil {
					return
				}
				if _, err := (&relay_protocol.Response{Version: relay_protocol.Version1, Status: relay_protocol.StatusOK}).WriteTo(c); err != nil {
					return
				}
				p := make([]byte, len(request))
				if _, err := io.ReadFull(c, p); err == nil && bytes.Equal(p, request) {
					c.Write(response)
				}
			}()
		}
	}()
	traffic := &Traffic{}
	primary, err := StartGost("socks5://127.0.0.1:0", "relay+tcp://"+upstream.Addr().String(), traffic)
	if err != nil {
		t.Fatal(err)
	}
	defer primary.Close()
	registered := append([]string{}, primary.connectors...)
	addr := primary.svcs[0].Addr().String()
	dialer, err := xproxy.SOCKS5("tcp", addr, nil, &net.Dialer{Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	exchange := func(c net.Conn) {
		t.Helper()
		defer c.Close()
		c.SetDeadline(time.Now().Add(3 * time.Second))
		if _, err := c.Write(request); err != nil {
			t.Fatal(err)
		}
		p := make([]byte, len(response))
		if _, err := io.ReadFull(c, p); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(p, response) {
			t.Fatal("payload changed")
		}
	}
	c, err := dialer.Dial("tcp", "example.invalid:443")
	if err != nil {
		t.Fatal(err)
	}
	exchange(c)
	if got := traffic.Snapshot(); got != (TrafficSnapshot{Upload: uint64(len(request)), Download: uint64(len(response))}) {
		t.Fatalf("incorrect direction or handshake counted: %+v", got)
	}
	localRelay, err := StartGost("http://127.0.0.1:0", "socks5://"+addr+"?notls=true")
	if err != nil {
		t.Fatal(err)
	}
	defer localRelay.Close()
	raw, err := net.DialTimeout("tcp", localRelay.svcs[0].Addr().String(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	raw.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.WriteString(raw, "CONNECT example.invalid:443 HTTP/1.1\r\nHost: example.invalid:443\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(raw), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatal(resp.Status)
	}
	exchange(raw)
	if got := traffic.Snapshot(); got != (TrafficSnapshot{Upload: uint64(2 * len(request)), Download: uint64(2 * len(response))}) {
		t.Fatalf("local relay must be counted exactly once: %+v", got)
	}
	primary.Close()
	for _, name := range registered {
		if registry.ConnectorRegistry().Get(name) != nil {
			t.Fatal("metered connector registration leaked")
		}
	}
}

type partialTrafficConn struct{ net.Conn }

func (*partialTrafficConn) Read(p []byte) (int, error)  { return copy(p, "abc"), io.EOF }
func (*partialTrafficConn) Write(p []byte) (int, error) { return 2, io.ErrShortWrite }

func TestTrafficPartialIOAndSessionIsolation(t *testing.T) {
	old := &Traffic{}
	c := &trafficConn{Conn: &partialTrafficConn{}, traffic: old}
	if n, err := c.Write([]byte("abcdef")); n != 2 || !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(n, err)
	}
	if n, err := c.Read(make([]byte, 8)); n != 3 || !errors.Is(err, io.EOF) {
		t.Fatal(n, err)
	}
	want := TrafficSnapshot{Upload: 2, Download: 3}
	if got := old.Snapshot(); got != want {
		t.Fatal(got)
	}
	old.Freeze()
	next := &Traffic{}
	c.Write([]byte("late"))
	c.Read(make([]byte, 8))
	if old.Snapshot() != want || next.Snapshot() != (TrafficSnapshot{}) {
		t.Fatal("late I/O changed closed/new session")
	}
}

type fakeTrafficUDP struct{ fail bool }

func (c *fakeTrafficUDP) Send([]byte, string) error {
	if c.fail {
		return io.ErrClosedPipe
	}
	return nil
}
func (c *fakeTrafficUDP) Receive() ([]byte, string, error) {
	if c.fail {
		return []byte("ignored"), "", io.EOF
	}
	return []byte("reply"), "example.invalid:53", nil
}
func (*fakeTrafficUDP) Close() error { return nil }

func TestTrafficUDPPayloadAndErrors(t *testing.T) {
	meter := &Traffic{}
	base := &fakeTrafficUDP{}
	c := &trafficHyUDP{HyUDPConn: base, traffic: meter}
	c.Send([]byte("query"), "example.invalid:53")
	c.Receive()
	base.fail = true
	c.Send([]byte("failed"), "example.invalid:53")
	c.Receive()
	if got := meter.Snapshot(); got != (TrafficSnapshot{Upload: 5, Download: 5}) {
		t.Fatal(got)
	}
}
