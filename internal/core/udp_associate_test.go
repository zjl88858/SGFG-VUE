package core

import (
	"bytes"
	"io"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	hyclient "github.com/zjl88858/oh-my-hysteria/core/v2/client"
)

type udpTestPacket struct {
	data []byte
	addr string
}

type udpTestConn struct {
	sent    chan udpTestPacket
	replies chan udpTestPacket
	done    chan struct{}
	once    sync.Once
}

func newUDPTestConn() *udpTestConn {
	return &udpTestConn{sent: make(chan udpTestPacket, 32), replies: make(chan udpTestPacket, 32), done: make(chan struct{})}
}

func (c *udpTestConn) Send(p []byte, addr string) error {
	select {
	case c.sent <- udpTestPacket{data: append([]byte{}, p...), addr: addr}:
		return nil
	case <-c.done:
		return io.ErrClosedPipe
	}
}

func (c *udpTestConn) Receive() ([]byte, string, error) {
	select {
	case p := <-c.replies:
		return p.data, p.addr, nil
	case <-c.done:
		return nil, "", io.EOF
	}
}

func (c *udpTestConn) Close() error { c.once.Do(func() { close(c.done) }); return nil }

type udpTestClient struct {
	udp   *udpTestConn
	calls atomic.Int32
}

func (*udpTestClient) TCP(string) (net.Conn, error)       { return nil, io.ErrClosedPipe }
func (c *udpTestClient) UDP() (hyclient.HyUDPConn, error) { c.calls.Add(1); return c.udp, nil }
func (c *udpTestClient) Close() error                     { return c.udp.Close() }

// Full local SOCKS handshake, but the upstream is an in-memory fake. These tests
// never load the driver, launch/elevate a helper or contact an Internet node.
func startTestUDPAssociate(t *testing.T, requested string, hy *udpTestClient) (netip.AddrPort, byte) {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := ln.Accept()
		if err == nil {
			handleSocks5(c, hy)
		}
	}()
	c, err := net.DialTimeout("tcp4", ln.Addr().String(), 2*time.Second)
	if err != nil {
		ln.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c.Close()
		ln.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("UDP association did not stop with its TCP control connection")
		}
	})
	c.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.Write([]byte{5, 1, 0}); err != nil {
		t.Fatal(err)
	}
	var auth [2]byte
	if _, err := io.ReadFull(c, auth[:]); err != nil || auth != [2]byte{5, 0} {
		t.Fatal("SOCKS auth", auth, err)
	}
	req := buildUDPRelayHeader(requested)
	req[0], req[1] = 5, 3
	if _, err := c.Write(req); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 512)
	if _, err := io.ReadFull(c, buf[:4]); err != nil {
		t.Fatal(err)
	}
	if buf[0] != 5 || buf[2] != 0 {
		t.Fatal("invalid SOCKS reply", buf[:4])
	}
	rep := buf[1]
	addr, err := readSocksAddr(c, buf)
	if err != nil {
		t.Fatal(err)
	}
	ap, err := netip.ParseAddrPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	c.SetDeadline(time.Time{})
	return ap, rep
}

func listenTestUDP(t *testing.T) *net.UDPConn {
	t.Helper()
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestUDPAssociatePinnedEndpointPreventsHijack(t *testing.T) {
	for _, wildcardIP := range []bool{false, true} {
		name := "explicit-IP"
		if wildcardIP {
			name = "unspecified-IP-with-declared-port"
		}
		t.Run(name, func(t *testing.T) {
			victim, attacker := listenTestUDP(t), listenTestUDP(t)
			requested := victim.LocalAddr().(*net.UDPAddr).AddrPort()
			if wildcardIP {
				requested = netip.AddrPortFrom(netip.IPv4Unspecified(), requested.Port())
			}
			upstream := newUDPTestConn()
			hy := &udpTestClient{udp: upstream}
			bound, rep := startTestUDPAssociate(t, requested.String(), hy)
			if rep != 0 {
				t.Fatalf("UDP ASSOCIATE rejected: %d", rep)
			}
			if hy.calls.Load() != 1 {
				t.Fatal("unexpected upstream associations")
			}
			const target = "192.0.2.10:443"
			send := func(c *net.UDPConn, p []byte) {
				t.Helper()
				if _, err := c.WriteToUDPAddrPort(p, bound); err != nil {
					t.Fatal(err)
				}
			}
			reply := func(text string) {
				t.Helper()
				upstream.replies <- udpTestPacket{data: []byte(text), addr: target}
				victim.SetReadDeadline(time.Now().Add(2 * time.Second))
				p := make([]byte, 1024)
				n, from, err := victim.ReadFromUDPAddrPort(p)
				if err != nil {
					t.Fatal("reply stolen or lost", err)
				}
				addr, hdr, err := parseUDPRelayHeader(p[:n])
				if err != nil || addr != target || !bytes.Equal(p[hdr:n], []byte(text)) || from != bound {
					t.Fatal("incorrect reply", addr, from, err)
				}
			}
			expectSent := func(text string) {
				t.Helper()
				select {
				case p := <-upstream.sent:
					if string(p.data) != text || p.addr != target {
						t.Fatalf("foreign/invalid packet forwarded: %+v", p)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("legitimate packet not forwarded")
				}
			}
			packet := func(text string) []byte { return append(buildUDPRelayHeader(target), []byte(text)...) }
			// The client endpoint is fixed at handshake time, even before its
			// first valid datagram. Invalid foreign packets cannot prime it.
			send(attacker, []byte{0})
			send(attacker, []byte{0, 0, 0, 99})
			reply("before-first-send")
			// Same IP but a different UDP source port: valid spoofed traffic must
			// be rejected as well, not merely prevented from updating an address.
			send(attacker, packet("injected-before"))
			send(victim, packet("victim-first"))
			expectSent("victim-first")
			reply("first-response")
			// Neither malformed nor well-formed later traffic may steal replies.
			send(attacker, nil)
			send(attacker, packet("injected-after"))
			badRSV := packet("bad-reserved")
			badRSV[0] = 1
			send(victim, badRSV)
			fragment := packet("fragmented")
			fragment[2] = 1
			send(victim, fragment)
			send(victim, packet("victim-second"))
			expectSent("victim-second")
			reply("second-response")
			attacker.SetReadDeadline(time.Now().Add(60 * time.Millisecond))
			if n, _, err := attacker.ReadFromUDPAddrPort(make([]byte, 1024)); err == nil {
				t.Fatalf("attacker received %d bytes", n)
			} else if e, ok := err.(net.Error); !ok || !e.Timeout() {
				t.Fatal(err)
			}
			select {
			case p := <-upstream.sent:
				t.Fatalf("unexpected upstream packet: %+v", p)
			default:
			}
		})
	}
}

func TestUDPAssociateRejectsUnpinnableEndpoint(t *testing.T) {
	for _, requested := range []string{"0.0.0.0:0", "127.0.0.1:0", "127.0.0.2:12345", "192.0.2.1:12345", "example.invalid:12345"} {
		t.Run(requested, func(t *testing.T) {
			hy := &udpTestClient{udp: newUDPTestConn()}
			_, rep := startTestUDPAssociate(t, requested, hy)
			if rep != 2 {
				t.Fatalf("expected policy rejection, got %d", rep)
			}
			if hy.calls.Load() != 0 {
				t.Fatal("invalid association opened an upstream")
			}
		})
	}
}

func TestUDPAssociatePeerNormalization(t *testing.T) {
	for _, tc := range []struct{ peer, requested, want string }{
		{"127.0.0.1:1111", "127.0.0.1:2222", "127.0.0.1:2222"},
		{"127.0.0.1:1111", "0.0.0.0:2222", "127.0.0.1:2222"},
		{"[::ffff:127.0.0.1]:1111", "127.0.0.1:2222", "127.0.0.1:2222"},
		{"127.0.0.1:1111", "[::ffff:127.0.0.1]:2222", "127.0.0.1:2222"},
		{"[::1]:1111", "[::]:2222", "[::1]:2222"},
		{"192.0.2.1:1111", "0.0.0.0:2222", "192.0.2.1:2222"},
	} {
		peer, err := net.ResolveTCPAddr("tcp", tc.peer)
		if err != nil {
			t.Fatal(err)
		}
		got, err := udpAssociateClient(peer, tc.requested)
		if err != nil || got.String() != tc.want {
			t.Fatalf("%s / %s: got %s, %v", tc.peer, tc.requested, got, err)
		}
	}
}
