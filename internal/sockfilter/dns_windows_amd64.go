//go:build windows && amd64

package sockfilter

import (
	"context"
	"fmt"
	"unsafe"
)

type dnsEndpoint struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// Endpoint cancellation prevents posting stale replies after udpClosed.
// Both endpoint count and total concurrent TCP queries are bounded.
func (s *Session) dnsSend(id uintptr, original address, data unsafe.Pointer, size uintptr) {
	packet := append([]byte(nil), unsafe.Slice((*byte)(data), int(size))...)
	q, _, err := dnsQuery(packet)
	if err != nil {
		return
	}
	s.mu.Lock()
	if s.closing || s.dnsPending >= 64 {
		s.mu.Unlock()
		return
	}
	if s.dns == nil {
		s.dns = make(map[uintptr]*dnsEndpoint)
	}
	f := s.dns[id]
	if f == nil {
		if len(s.dns) >= 512 {
			s.mu.Unlock()
			return
		}
		ctx, cancel := context.WithCancel(s.ctx)
		f = &dnsEndpoint{ctx, cancel}
		s.dns[id] = f
	}
	s.dnsPending++
	s.wg.Add(1)
	s.mu.Unlock()
	if n := s.dnsQueries.Add(1); s.debug && n <= 5 {
		s.log("info", "SockFilter DNS: UDP 查询 → 经节点 TCP 1.1.1.1:53")
	}
	go func() {
		defer s.wg.Done()
		defer func() { s.mu.Lock(); s.dnsPending--; s.mu.Unlock() }()
		reply, err := exchangeDNS(f.ctx, s.proxy, packet)
		if f.ctx.Err() != nil {
			return
		}
		if err != nil {
			if s.dnsFailures.Add(1) <= 5 {
				s.log("warn", fmt.Sprintf("DNS 解析失败: %v", err))
			}
			reply = dnsFailure(q)
		}
		if len(reply) > 0 {
			// Return the ORIGINAL resolver address, not 1.1.1.1, so connected
			// UDP sockets and callers validating the DNS peer accept the reply.
			s.receiveUDP(id, original, reply)
			s.dnsReplies.Add(1)
		}
	}()
}
