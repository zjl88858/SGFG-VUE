//go:build windows && amd64

package sockfilter

import (
	"context"
	"net/netip"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/net/dns/dnsmessage"
)

func TestDNSUDPOnTCPOnlyUpstream(t *testing.T) {
	proxy := fakeDNSSOCKS(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{ctx: ctx, cancel: cancel, proxy: proxy, tcpOnly: true, log: func(string, string) {}}
	defer func() { cancel(); s.wg.Wait() }()
	type received struct {
		id   uintptr
		from address
		data []byte
	}
	got := make(chan received, 3)
	s.receiveUDP = func(id uintptr, a address, b []byte) { got <- received{id, a, append([]byte(nil), b...)} }
	// api is nil: proves DNS does not enter bypass/nf_udpPostSend or the
	// normal UDP association path, even for a loopback resolver.
	for i, addr := range []string{"127.0.0.1:53", "192.168.50.1:53", "[::1]:53"} {
		a := fromAddrPort(netip.MustParseAddrPort(addr))
		q := testDNSQuery(t, uint16(i+1))
		s.udpSend(uintptr(i+1), &a, unsafe.Pointer(&q[0]), uintptr(len(q)), nil)
		q[0] = 255 // original callback buffer may be reused immediately
		select {
		case r := <-got:
			var msg dnsmessage.Message
			if r.from != a || r.id != uintptr(i+1) || msg.Unpack(r.data) != nil || msg.ID != uint16(i+1) || len(msg.Answers) != 1 {
				t.Fatal("wrong original peer/ID/reply")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("missing DNS response")
		}
	}
	if s.udpSOCKS.Load() != 0 || s.dnsQueries.Load() != 3 {
		t.Fatal("DNS used UDP upstream")
	}
	s.udpClosed(1)
	s.mu.Lock()
	_, exists := s.dns[1]
	s.mu.Unlock()
	if exists {
		t.Fatal("DNS endpoint was not removed")
	}
	// Non-DNS UDP on a TCP-only transport is dropped, not sent directly.
	a := fromAddrPort(netip.MustParseAddrPort("1.2.3.4:443"))
	b := []byte("quic")
	s.udpSend(4, &a, unsafe.Pointer(&b[0]), uintptr(len(b)), nil)
	if s.udpDropped.Load() != 1 {
		t.Fatal("unsupported UDP not dropped")
	}
}
