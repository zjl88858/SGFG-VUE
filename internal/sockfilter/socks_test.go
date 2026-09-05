package sockfilter

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"
)

func TestSocksAddress(t *testing.T) {
	for _, value := range []string{"1.2.3.4:80", "[2001:db8::5]:443"} {
		want := netip.MustParseAddrPort(value)
		got, err := readSocksAddress(bytes.NewReader(socksAddress(want)))
		if err != nil || got != want {
			t.Fatalf("%v %v", got, err)
		}
	}
	for _, b := range [][]byte{{}, {1, 2}, {4, 0}, {3, 1, 'x', 0, 80}, {9}} {
		if _, err := readSocksAddress(bytes.NewReader(b)); err == nil {
			t.Errorf("accepted invalid address: %v", b)
		}
	}
}

// Local-only integration test for both command types, including fragmented
// SOCKS responses. No DLL/driver, upstream nodes or external network involved.
func TestSOCKSHandshake(t *testing.T) {
	for _, cmd := range []byte{1, 3} {
		t.Run(map[byte]string{1: "CONNECT", 3: "UDP_ASSOCIATE"}[cmd], func(t *testing.T) {
			ln, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			target := netip.MustParseAddrPort("[2001:db8::1]:443")
			bound := netip.MustParseAddrPort("127.0.0.1:12345")
			done := make(chan error, 1)
			go func() {
				c, err := ln.Accept()
				if err != nil {
					done <- err
					return
				}
				defer c.Close()
				c.SetDeadline(time.Now().Add(3 * time.Second))
				var greeting [3]byte
				if _, err := io.ReadFull(c, greeting[:]); err != nil {
					done <- err
					return
				}
				if greeting != [3]byte{5, 1, 0} {
					done <- io.ErrUnexpectedEOF
					return
				}
				c.Write([]byte{5})
				c.Write([]byte{0})
				var request [3]byte
				if _, err := io.ReadFull(c, request[:]); err != nil {
					done <- err
					return
				}
				ap, err := readSocksAddress(c)
				if err != nil {
					done <- err
					return
				}
				if ap != target || request != [3]byte{5, cmd, 0} {
					done <- io.ErrUnexpectedEOF
					return
				}
				for _, b := range append([]byte{5, 0, 0}, socksAddress(bound)...) {
					c.Write([]byte{b})
				}
				done <- nil
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			c, ap, err := openSOCKS(ctx, ln.Addr().String(), cmd, target)
			if err != nil {
				t.Fatal(err)
			}
			c.Close()
			if ap != bound {
				t.Fatal(ap)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSOCKSCancellation(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(3 * time.Second))
		io.Copy(io.Discard, c) // Deliberately never respond to the greeting.
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if c, _, err := openSOCKS(ctx, ln.Addr().String(), 1, netip.MustParseAddrPort("1.1.1.1:443")); err == nil {
		c.Close()
		t.Fatal("expected cancellation")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("cancellation did not interrupt handshake")
	}
	<-done
}

func TestSOCKSReject(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(3 * time.Second))
		var b [3]byte
		io.ReadFull(c, b[:])
		c.Write([]byte{5, 255})
	}()
	if c, _, err := openSOCKS(context.Background(), ln.Addr().String(), 1, netip.MustParseAddrPort("1.1.1.1:443")); err == nil {
		c.Close()
		t.Fatal("auth refusal ignored")
	}
	<-done
}
