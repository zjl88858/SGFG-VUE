//go:build windows && amd64

package sockfilter

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestCallbackABIWithoutDriver(t *testing.T) {
	initCallbacks()
	before := callbacks
	initCallbacks()
	if before != callbacks {
		t.Fatal("callback table changed on reconnect")
	}
	argc := []int{0, 0, 2, 2, 2, 3, 3, 1, 1, 2, 2, 2, 5, 5, 1, 1}
	for i, cb := range callbacks {
		if cb == 0 {
			t.Fatal("missing callback", i)
		}
		// active is nil: validates marshaling/return ABI without loading a DLL.
		r, _, _ := syscall.SyscallN(cb, make([]uintptr, argc[i])...)
		if r != 0 {
			t.Fatal("unexpected return", i, r)
		}
	}
}

// Regression: an ordinary selected client must be redirected without invoking
// nf_tcpIsProxy (which returned -1 in the supplied DLL). api is deliberately nil.
func TestSelectedTCPDoesNotUseProxyProbe(t *testing.T) {
	ln, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	s := &Session{listeners: map[uint16]*net.TCPListener{2: ln}, targets: make(map[netip.AddrPort]tcpTarget), tcpIDs: make(map[uintptr]netip.AddrPort), log: func(string, string) {}}
	var info [67]byte
	binary.LittleEndian.PutUint32(info[:4], 16)
	binary.LittleEndian.PutUint32(info[4:8], uint32(os.Getpid())+1)
	binary.LittleEndian.PutUint16(info[9:11], 2)
	local := fromAddrPort(netip.MustParseAddrPort("0.0.0.0:12345"))
	remote := fromAddrPort(netip.MustParseAddrPort("192.0.2.1:443"))
	copy(info[11:39], local[:])
	copy(info[39:67], remote[:])
	s.tcpConnect(1, &info)
	to, _ := address(info[39:67]).addrPort()
	if !to.Addr().IsLoopback() || to.Port() != uint16(ln.Addr().(*net.TCPAddr).Port) {
		t.Fatalf("not redirected: %s", to)
	}
	if binary.LittleEndian.Uint32(info[4:8]) != uint32(os.Getpid()) {
		t.Fatal("missing self PID")
	}
	key := netip.MustParseAddrPort("127.0.0.1:12345")
	if s.targets[key].remote != netip.MustParseAddrPort("192.0.2.1:443") {
		t.Fatal("lost original destination")
	}
}

func TestUDPForwardingWithoutDriver(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	u, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer u.Close()
	u.SetDeadline(time.Now().Add(3 * time.Second))
	target := netip.MustParseAddrPort("[2001:db8::2]:443")
	bound := u.LocalAddr().(*net.UDPAddr).AddrPort()
	bound = netip.AddrPortFrom(bound.Addr().Unmap(), bound.Port())
	serverErr := make(chan error, 1)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		c, err := ln.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(3 * time.Second))
		var b [3]byte
		if _, err := io.ReadFull(c, b[:]); err != nil {
			serverErr <- err
			return
		}
		c.Write([]byte{5, 0})
		if _, err := io.ReadFull(c, b[:]); err != nil {
			serverErr <- err
			return
		}
		client, err := readSocksAddress(c)
		if err != nil || b != [3]byte{5, 3, 0} || client.Port() == 0 {
			serverErr <- fmt.Errorf("bad association: %v %v", client, err)
			return
		}
		c.Write(append([]byte{5, 0, 0}, socksAddress(bound)...))
		buf := make([]byte, 1024)
		n, from, err := u.ReadFromUDPAddrPort(buf)
		if err != nil {
			serverErr <- err
			return
		}
		reader := bytes.NewReader(buf[3:n])
		ap, err := readSocksAddress(reader)
		payload, _ := io.ReadAll(reader)
		if err != nil || ap != target || string(payload) != "hello" {
			serverErr <- fmt.Errorf("bad UDP frame: %s %q %v", ap, payload, err)
			return
		}
		u.WriteToUDPAddrPort(append(append([]byte{0, 0, 0}, socksAddress(target)...), []byte("reply")...), from)
		serverErr <- nil
		io.Copy(io.Discard, c) // Keep the control connection alive until canceled.
	}()
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{ctx: ctx, cancel: cancel, proxy: ln.Addr().String(), udp: make(map[uintptr]*udpFlow), log: func(string, string) {}}
	got := make(chan udpPacket, 1)
	s.receiveUDP = func(id uintptr, a address, payload []byte) {
		ap, err := a.addrPort()
		if id != 123 || err != nil {
			return
		}
		got <- udpPacket{ap, append([]byte(nil), payload...)}
	}
	defer func() { cancel(); s.wg.Wait(); ln.Close(); <-serverDone }()
	a := fromAddrPort(target)
	payload := []byte("hello")
	s.udpSend(123, &a, unsafe.Pointer(&payload[0]), uintptr(len(payload)), nil)
	copy(payload, "xxxxx") // Simulate SDK reusing its callback buffer.
	select {
	case p := <-got:
		if p.addr != target || string(p.data) != "reply" {
			t.Fatalf("bad receive: %+v", p)
		}
	case err := <-serverErr:
		if err != nil {
			t.Fatal(err)
		}
		select {
		case p := <-got:
			if p.addr != target || string(p.data) != "reply" {
				t.Fatal(p)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("missing reply")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("UDP forwarding timed out")
	}
	cancel()
	s.wg.Wait()
	if s.queuedUDPBytes != 0 {
		t.Fatal("queued memory was not released")
	}
}
