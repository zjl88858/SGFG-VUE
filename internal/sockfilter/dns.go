package sockfilter

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/netip"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

var dnsUpstream = netip.MustParseAddrPort("1.1.1.1:53")

// Options describes capabilities of the selected upstream, not the protocol
// used by the local SOCKS5 listener. DNS always uses SOCKS5 CONNECT (TCP).
type Options struct{ TCPOnly bool }

func tcpDestination(original netip.AddrPort) netip.AddrPort {
	if original.Port() == 53 {
		return dnsUpstream
	}
	return original
}

func dnsQuery(packet []byte) (dnsmessage.Message, int, error) {
	var q dnsmessage.Message
	if len(packet) < 12 || len(packet) > 65535 {
		return q, 0, fmt.Errorf("invalid DNS message length")
	}
	if err := q.Unpack(packet); err != nil {
		return q, 0, err
	}
	if q.Response || q.OpCode != 0 || len(q.Questions) != 1 {
		return q, 0, fmt.Errorf("expected a standard single-question DNS query")
	}
	limit := 512
	for _, rr := range q.Additionals {
		if rr.Header.Type == dnsmessage.TypeOPT {
			limit = max(512, min(int(rr.Header.Class), 65507))
			break
		}
	}
	return q, limit, nil
}

func dnsFailure(q dnsmessage.Message) []byte {
	m := dnsmessage.Message{Header: dnsmessage.Header{
		ID: q.ID, Response: true, RecursionDesired: q.RecursionDesired,
		RecursionAvailable: true, CheckingDisabled: q.CheckingDisabled, RCode: dnsmessage.RCodeServerFailure,
	}, Questions: q.Questions}
	b, _ := m.Pack()
	return b
}

// exchangeDNS converts one UDP query into DNS-over-TCP inside the selected
// proxy tunnel. There is no direct dial to the resolver and no UDP association.
func exchangeDNS(ctx context.Context, proxy string, packet []byte) ([]byte, error) {
	q, limit, err := dnsQuery(packet)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	c, _, err := openSOCKS(ctx, proxy, 1, dnsUpstream)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	c.SetDeadline(deadline)
	frame := binary.BigEndian.AppendUint16(nil, uint16(len(packet)))
	frame = append(frame, packet...)
	if _, err := io.Copy(c, bytes.NewReader(frame)); err != nil {
		return nil, err
	}
	var prefix [2]byte
	if _, err := io.ReadFull(c, prefix[:]); err != nil {
		return nil, err
	}
	response := make([]byte, int(binary.BigEndian.Uint16(prefix[:])))
	if _, err := io.ReadFull(c, response); err != nil {
		return nil, err
	}
	var r dnsmessage.Message
	if err := r.Unpack(response); err != nil {
		return nil, err
	}
	if !r.Response || r.ID != q.ID || r.OpCode != q.OpCode || len(r.Questions) != 1 ||
		!strings.EqualFold(r.Questions[0].Name.String(), q.Questions[0].Name.String()) ||
		r.Questions[0].Type != q.Questions[0].Type || r.Questions[0].Class != q.Questions[0].Class {
		return nil, fmt.Errorf("DNS reply does not match query")
	}
	if len(response) > limit {
		// Preserve the client's advertised UDP size. TC causes a TCP retry,
		// whose port 53 connection is also redirected to the fixed resolver.
		r.Truncated = true
		r.Answers = nil
		r.Authorities = nil
		r.Additionals = nil
		return r.Pack()
	}
	return response, nil
}
