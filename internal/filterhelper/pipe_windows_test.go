//go:build windows && amd64

package filterhelper

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Exercises real local IPC/ACL/deadlines as an ordinary user. No elevation,
// helper process, SDK, driver registration or public network traffic.
func TestWindowsPipeIdentityAndDeadlines(t *testing.T) {
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	name := pipePrefix + hex.EncodeToString(nonce[:])
	h, err := createPipe(name)
	if err != nil {
		t.Fatal(err)
	}
	owned := true
	defer func() {
		if owned {
			windows.CloseHandle(h)
		}
	}()
	clientCh := make(chan *os.File, 1)
	errorCh := make(chan error, 1)
	go func() {
		f, e := dialPipe(name)
		if e != nil {
			errorCh <- e
			return
		}
		clientCh <- f
	}()
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(process)
	if err := acceptPipe(h, process); err != nil {
		t.Fatal(err)
	}
	var client *os.File
	select {
	case client = <-clientCh:
	case err := <-errorCh:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("client blocked")
	}
	defer client.Close()
	if err := verifyPeer(h, uint32(os.Getpid()), false); err != nil {
		t.Fatal(err)
	}
	if err := verifyPeer(windows.Handle(client.Fd()), uint32(os.Getpid()), true); err != nil {
		t.Fatal(err)
	}
	if err := verifyPeer(h, uint32(os.Getpid()+1), false); err == nil {
		t.Fatal("wrong client accepted")
	}
	server := os.NewFile(uintptr(h), name)
	owned = false
	defer server.Close()
	w := &wire{connection: server}
	done := make(chan error, 1)
	go func() { done <- (&wire{connection: client}).send(message{Type: "ping"}) }()
	if m, err := w.receive(time.Second); err != nil || m.Type != "ping" {
		t.Fatal(m, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := w.receive(30 * time.Millisecond); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("pipe read deadline unsupported: %v", err)
	}
	go func() { done <- (&wire{connection: client}).send(message{Type: "stop"}) }()
	if m, err := w.receive(time.Second); err != nil || m.Type != "stop" {
		t.Fatal("pipe cannot resume after deadline", m, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestShellExecuteLayout(t *testing.T) {
	var s shellExecuteInfo
	if unsafe.Sizeof(s) != 112 || unsafe.Offsetof(s.Process) != 104 {
		t.Fatal("invalid SHELLEXECUTEINFOW ABI")
	}
}
