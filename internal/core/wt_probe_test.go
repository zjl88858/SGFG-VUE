package core

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/proxy"
)

// Explicit opt-in: one DNS-over-TCP query through one real node, no driver.
// Build as sockfilter-driver-tests.exe to reuse the user's Proxifier Direct rule.
func TestWebTransportProbe(t *testing.T) {
	if os.Getenv("SGFG_WT_PROBE") != "1" {
		t.Skip("manual real-node probe")
	}
	name := os.Getenv("SGFG_WT_NODE")
	if name == "" {
		name = "sky"
	}
	n, ok := NodeByName(name)
	if !ok {
		t.Fatal("unknown node name")
	}
	forward := fmt.Sprintf("relay+wt://%s@%s:%d?secure=true", url.UserPassword("dwg", n.Password), n.Host, GostServerPort)
	g, err := StartGost("socks5://127.0.0.1:0", forward)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	d, err := proxy.SOCKS5("tcp", g.svcs[0].Addr().String(), nil, &net.Dialer{Timeout: 8 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	c, err := d.(proxy.ContextDialer).DialContext(ctx, "tcp", "1.1.1.1:53")
	if err != nil {
		t.Fatalf("SOCKS CONNECT via %s: %v", name, err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(5 * time.Second))
	q := dnsmessage.Message{Header: dnsmessage.Header{ID: 1, RecursionDesired: true}, Questions: []dnsmessage.Question{{Name: dnsmessage.MustNewName("example.com."), Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}}}
	b, _ := q.Pack()
	frame := binary.BigEndian.AppendUint16(nil, uint16(len(b)))
	frame = append(frame, b...)
	if _, err := c.Write(frame); err != nil {
		t.Fatal(err)
	}
	var prefix [2]byte
	if _, err := io.ReadFull(c, prefix[:]); err != nil {
		t.Fatalf("DNS response via %s: %v", name, err)
	}
	b = make([]byte, int(binary.BigEndian.Uint16(prefix[:])))
	if _, err := io.ReadFull(c, b); err != nil {
		t.Fatal(err)
	}
	var reply dnsmessage.Message
	if err := reply.Unpack(b); err != nil {
		t.Fatal(err)
	}
	if !reply.Response || reply.ID != 1 || reply.RCode != dnsmessage.RCodeSuccess {
		t.Fatalf("invalid DNS response: %+v", reply.Header)
	}
	t.Logf("node=%s DNS-over-TCP reply received, answers=%d", name, len(reply.Answers))
}
