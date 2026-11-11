package core

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	omclient "github.com/zjl88858/oh-my-hysteria/core/v2/client"
	omserver "github.com/zjl88858/oh-my-hysteria/core/v2/server"
	"github.com/zjl88858/oh-my-hysteria/extras/v2/obfs"
)

func omhyTestNode() Node {
	padding, junk := 16, 3
	b := BlinkConfig{Password: "local-test-blink-key-0123456789", MaxPacketSize: 1150, Fragmentation: true, PaddingMax: &padding, JunkPackets: &junk, MaxSessions: 3072}
	return Node{Name: "test", Host: "127.0.0.1", Password: "test-auth", DirectBlink: b, TurtleorBlink: b}
}

func TestOMHYConfigUsesCOPAAndModePorts(t *testing.T) {
	for _, mode := range []Transport{TransportOMHYDirect, TransportOMHYTurtleor} {
		sel, err := BuildSelection(mode, omhyTestNode())
		if err != nil {
			t.Fatal(err)
		}
		factory := &blinkConnFactory{options: sel.BlinkConfig().options()}
		cfg, err := omhyConfig(context.Background(), sel, sel.Port, factory)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.CongestionConfig.Type != "copa" || cfg.BandwidthConfig.MaxTx != 0 || cfg.BandwidthConfig.MaxRx != 0 || cfg.TLSConfig.InsecureSkipVerify || cfg.TLSConfig.ServerName != "127.0.0.1" {
			t.Fatal("unsafe or incorrect client configuration")
		}
		if _, err := omhyConfig(context.Background(), sel, 443, factory); err == nil {
			t.Fatal("out-of-range port accepted")
		}
	}
}

func TestBlinkFactoryPreservesPacketLimitAndCancelsReads(t *testing.T) {
	f := &blinkConnFactory{options: omhyTestNode().DirectBlink.options()}
	c, err := f.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	limit, ok := c.(interface{ QUICPacketSizeLimit() int })
	if !ok || limit.QUICPacketSizeLimit() != 1200 {
		t.Fatal("Blink framing contract hidden from QUIC")
	}
	done := make(chan error, 1)
	go func() { _, _, err := c.ReadFrom(make([]byte, 1200)); done <- err }()
	f.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed packet socket readable")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("factory close did not cancel pending read")
	}
	c.Close()
	if _, err := f.New(nil); !errors.Is(err, net.ErrClosed) {
		t.Fatal("factory reopened after Close", err)
	}
	f.mu.Lock()
	remaining := len(f.conns)
	f.mu.Unlock()
	if remaining != 0 {
		t.Fatal("UDP socket leaked")
	}
}

func TestOMHYLazyListenerRepeatedStartClose(t *testing.T) {
	for _, mode := range []Transport{TransportOMHYDirect, TransportOMHYTurtleor} {
		for i := 0; i < 2; i++ {
			sel, err := BuildSelection(mode, omhyTestNode())
			if err != nil {
				t.Fatal(err)
			}
			h, err := StartOMHY(sel, "socks5", "127.0.0.1", 0, &Traffic{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			c, err := net.Dial("tcp", h.Addr())
			if err != nil {
				t.Fatal(err)
			}
			c.SetDeadline(time.Now().Add(2 * time.Second))
			c.Write([]byte{5, 1, 0})
			var auth [2]byte
			if _, err := io.ReadFull(c, auth[:]); err != nil || auth != [2]byte{5, 0} {
				t.Fatal("local SOCKS listener", err)
			}
			h.factory.mu.Lock()
			sockets := len(h.factory.conns)
			h.factory.mu.Unlock()
			if sockets != 0 {
				t.Fatal("lazy client contacted upstream before a request")
			}
			done := make(chan struct{})
			go func() { h.Close(); h.Close(); close(done) }()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("close blocked on incomplete local request")
			}
			c.Close()
		}
	}
}

func TestOMHYReconnectConfigKeepsModeRange(t *testing.T) {
	for _, mode := range []Transport{TransportOMHYDirect, TransportOMHYTurtleor} {
		sel, err := BuildSelection(mode, omhyTestNode())
		if err != nil {
			t.Fatal(err)
		}
		factory := &blinkConnFactory{options: sel.BlinkConfig().options()}
		source := omhyConfigSource(context.Background(), sel, factory)
		min, max := mode.portRange()
		for i := 0; i < 64; i++ {
			cfg, err := source()
			if err != nil {
				t.Fatal(err)
			}
			addr, ok := cfg.ServerAddr.(*net.UDPAddr)
			if !ok || addr.Port < min || addr.Port > max || cfg.ServerAddr.Network() != "udp" || cfg.CongestionConfig.Type != "copa" {
				t.Fatal("reconnect changed mode or enabled port hopping")
			}
			if i == 0 && addr.Port != sel.Port {
				t.Fatal("initial selected port ignored")
			}
		}
	}
}

func TestBlinkReconnectRetiresPreviousSocket(t *testing.T) {
	factory := &blinkConnFactory{options: omhyTestNode().DirectBlink.options()}
	defer factory.Close()
	first, err := factory.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := factory.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	factory.mu.Lock()
	sockets := len(factory.conns)
	factory.mu.Unlock()
	if sockets != 1 {
		t.Fatal("reconnect retained an old UDP socket")
	}
	if _, _, err := first.ReadFrom(make([]byte, 1200)); err == nil {
		t.Fatal("previous socket still active")
	}
}

func testOMHYCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "SGFG local test"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, roots
}

type omhyTestAuth struct{}

func (omhyTestAuth) Authenticate(_ net.Addr, auth string, _ uint64) (bool, string) {
	return auth == "test-auth", "local-test"
}

type captureBlinkFactory struct {
	inner *blinkConnFactory
	conn  net.PacketConn
}

func (f *captureBlinkFactory) New(addr net.Addr) (net.PacketConn, error) {
	c, err := f.inner.New(addr)
	f.conn = c
	return c, err
}

// Actual OMHY client/server, COPA, TLS and Blink on loopback. Outer packets
// are 1150 bytes with fragmentation and reassembly, matching the node examples.
// No public nodes, helper elevation, driver or Windows proxy settings are used.
func TestOMHYCOPABlinkLocalTCPAndUDP(t *testing.T) {
	cert, roots := testOMHYCertificate(t)
	for _, mode := range []Transport{TransportOMHYDirect, TransportOMHYTurtleor} {
		t.Run(string(mode), func(t *testing.T) {
			min, max := mode.portRange()
			var raw *net.UDPConn
			for port := min; port <= max; port++ {
				c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
				if err == nil {
					raw = c
					break
				}
			}
			if raw == nil {
				t.Skip("all three local mode ports are occupied")
			}
			defer raw.Close()
			node := omhyTestNode()
			sel := Selection{Transport: mode, Node: node, Port: raw.LocalAddr().(*net.UDPAddr).Port}
			serverOptions := sel.BlinkConfig().options()
			zero := 0
			serverOptions.Server = true
			serverOptions.JunkPackets = &zero
			serverConn, err := obfs.WrapPacketConnBlink(raw, serverOptions)
			if err != nil {
				t.Fatal(err)
			}
			defer serverConn.Close()
			srv, err := omserver.NewServer(&omserver.Config{Conn: serverConn, TLSConfig: omserver.TLSConfig{Certificates: []tls.Certificate{cert}}, CongestionConfig: omserver.CongestionConfig{Type: "copa"}, IgnoreClientBandwidth: true, Authenticator: omhyTestAuth{}})
			if err != nil {
				t.Fatal(err)
			}
			serveDone := make(chan error, 1)
			go func() { serveDone <- srv.Serve() }()
			defer func() {
				srv.Close()
				select {
				case <-serveDone:
				case <-time.After(2 * time.Second):
					t.Error("server did not close")
				}
			}()
			factory := &blinkConnFactory{options: sel.BlinkConfig().options()}
			defer factory.Close()
			capture := &captureBlinkFactory{inner: factory}
			cfg, err := omhyConfig(context.Background(), sel, sel.Port, factory)
			if err != nil {
				t.Fatal(err)
			}
			cfg.TLSConfig.RootCAs = roots
			cfg.ConnFactory = capture
			client, info, err := omclient.NewClient(cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if !info.UDPEnabled || info.Tx != 0 {
				t.Fatal("unexpected UDP/bandwidth negotiation")
			}
			traffic := &Traffic{}
			metered := &trafficHyClient{Client: client, traffic: traffic}
			ln, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			var peers sync.WaitGroup
			peers.Add(1)
			go func() {
				defer peers.Done()
				c, err := ln.Accept()
				if err != nil {
					return
				}
				defer c.Close()
				c.SetDeadline(time.Now().Add(3 * time.Second))
				io.Copy(c, c)
			}()
			stream, err := metered.TCP(ln.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			stream.SetDeadline(time.Now().Add(3 * time.Second))
			payload := bytes.Repeat([]byte("tcp-payload"), 200)
			if _, err := stream.Write(payload); err != nil {
				stream.Close()
				t.Fatal(err)
			}
			got := make([]byte, len(payload))
			if _, err := io.ReadFull(stream, got); err != nil {
				stream.Close()
				t.Fatal(err)
			}
			stream.Close()
			peers.Wait()
			if !bytes.Equal(got, payload) {
				t.Fatal("TCP payload changed")
			}
			udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				t.Fatal(err)
			}
			defer udp.Close()
			go func() {
				p := make([]byte, 65535)
				n, a, err := udp.ReadFromUDPAddrPort(p)
				if err == nil {
					udp.WriteToUDPAddrPort(p[:n], a)
				}
			}()
			association, err := metered.UDP()
			if err != nil {
				t.Fatal(err)
			}
			defer association.Close()
			udpPayload := bytes.Repeat([]byte("udp-payload"), 140)
			if err := association.Send(udpPayload, udp.LocalAddr().String()); err != nil {
				t.Fatal(err)
			}
			type udpResult struct {
				p   []byte
				err error
			}
			received := make(chan udpResult, 1)
			go func() { p, _, err := association.Receive(); received <- udpResult{p, err} }()
			select {
			case result := <-received:
				if result.err != nil || !bytes.Equal(result.p, udpPayload) {
					t.Fatal("UDP payload changed", result.err)
				}
			case <-time.After(4 * time.Second):
				t.Fatal("UDP reply timed out")
			}
			want := uint64(len(payload) + len(udpPayload))
			if got := traffic.Snapshot(); got.Upload != want || got.Download != want {
				t.Fatal("traffic boundary incorrect", got)
			}
			stats := capture.conn.(interface{ BlinkStats() obfs.BlinkStats }).BlinkStats()
			if stats.FragmentedMessages == 0 || stats.Reassembled == 0 || stats.JunkPackets != 3 {
				t.Fatal("full Blink framing/fragmentation/junk path not exercised")
			}
		})
	}
}
