package sockfilter

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func testDNSQuery(t *testing.T, id uint16) []byte {
	t.Helper()
	q := dnsmessage.Message{Header: dnsmessage.Header{ID: id, RecursionDesired: true}, Questions: []dnsmessage.Question{{Name: dnsmessage.MustNewName("example.test."), Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}}}
	b, err := q.Pack()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TCP-only SOCKS mock: a UDP ASSOCIATE or a direct connection to 1.1.1.1 is
// never needed. All tests remain local and do not load the driver.
func fakeDNSSOCKS(t *testing.T, change func(*dnsmessage.Message)) string {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer c.Close()
				c.SetDeadline(time.Now().Add(2 * time.Second))
				var b [3]byte
				if _, err := io.ReadFull(c, b[:]); err != nil {
					return
				}
				c.Write([]byte{5, 0})
				if _, err := io.ReadFull(c, b[:]); err != nil {
					return
				}
				ap, err := readSocksAddress(c)
				if err != nil || b != [3]byte{5, 1, 0} || ap != dnsUpstream {
					t.Errorf("expected CONNECT 1.1.1.1:53, got %v %s %v", b, ap, err)
					return
				}
				c.Write(append([]byte{5, 0, 0}, socksAddress(netip.MustParseAddrPort("127.0.0.1:1234"))...))
				var prefix [2]byte
				if _, err := io.ReadFull(c, prefix[:]); err != nil {
					return
				}
				packet := make([]byte, int(binary.BigEndian.Uint16(prefix[:])))
				if _, err := io.ReadFull(c, packet); err != nil {
					return
				}
				var q dnsmessage.Message
				if err := q.Unpack(packet); err != nil {
					t.Error(err)
					return
				}
				q.Response = true
				q.RecursionAvailable = true
				q.Answers = []dnsmessage.Resource{{Header: dnsmessage.ResourceHeader{Name: q.Questions[0].Name, Class: dnsmessage.ClassINET, TTL: 60}, Body: &dnsmessage.AResource{A: [4]byte{192, 0, 2, 1}}}}
				if change != nil {
					change(&q)
				}
				packet, err = q.Pack()
				if err != nil {
					t.Error(err)
					return
				}
				binary.BigEndian.PutUint16(prefix[:], uint16(len(packet)))
				c.Write(prefix[:1])
				c.Write(prefix[1:])
				c.Write(packet)
			}()
		}
	}()
	t.Cleanup(func() { ln.Close(); <-done; wg.Wait() })
	return ln.Addr().String()
}

func TestDNSTCPExchange(t *testing.T) {
	proxy := fakeDNSSOCKS(t, nil)
	raw, err := exchangeDNS(context.Background(), proxy, testDNSQuery(t, 42))
	if err != nil {
		t.Fatal(err)
	}
	var r dnsmessage.Message
	if err := r.Unpack(raw); err != nil {
		t.Fatal(err)
	}
	if r.ID != 42 || !r.Response || len(r.Answers) != 1 {
		t.Fatalf("bad DNS reply: %+v", r)
	}
}

func TestDNSRejectMismatch(t *testing.T) {
	proxy := fakeDNSSOCKS(t, func(m *dnsmessage.Message) { m.ID++ })
	if _, err := exchangeDNS(context.Background(), proxy, testDNSQuery(t, 1)); err == nil {
		t.Fatal("mismatched reply accepted")
	}
}

func TestDNSTruncationAndEDNS(t *testing.T) {
	proxy := fakeDNSSOCKS(t, func(m *dnsmessage.Message) {
		a := m.Answers[0]
		for i := 0; i < 40; i++ {
			m.Answers = append(m.Answers, a)
		}
	})
	for _, edns := range []bool{false, true} {
		q, _, _ := dnsQuery(testDNSQuery(t, 2))
		if edns {
			q.Additionals = []dnsmessage.Resource{{Header: dnsmessage.ResourceHeader{Name: dnsmessage.MustNewName("."), Class: 4096}, Body: &dnsmessage.OPTResource{}}}
		}
		packet, _ := q.Pack()
		raw, err := exchangeDNS(context.Background(), proxy, packet)
		if err != nil {
			t.Fatal(err)
		}
		var r dnsmessage.Message
		r.Unpack(raw)
		if edns {
			if r.Truncated || len(r.Answers) != 41 {
				t.Fatal("EDNS answer unexpectedly truncated")
			}
		} else {
			if !r.Truncated || len(raw) > 512 || len(r.Answers) != 0 {
				t.Fatal("oversized UDP reply was not truncated")
			}
		}
	}
}

func TestDNSFailureAndRouting(t *testing.T) {
	q, _, err := dnsQuery(testDNSQuery(t, 123))
	if err != nil {
		t.Fatal(err)
	}
	var r dnsmessage.Message
	if err := r.Unpack(dnsFailure(q)); err != nil {
		t.Fatal(err)
	}
	if r.ID != 123 || !r.Response || r.RCode != dnsmessage.RCodeServerFailure || !r.RecursionDesired {
		t.Fatal("bad SERVFAIL")
	}
	for _, addr := range []string{"127.0.0.1:53", "192.168.50.1:53", "[::1]:53"} {
		if tcpDestination(netip.MustParseAddrPort(addr)) != dnsUpstream {
			t.Fatal(addr)
		}
	}
	a := netip.MustParseAddrPort("1.2.3.4:443")
	if tcpDestination(a) != a {
		t.Fatal("non-DNS target changed")
	}
	if _, _, err := dnsQuery([]byte{1, 2}); err == nil {
		t.Fatal("malformed query accepted")
	}
}
