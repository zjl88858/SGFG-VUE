//go:build darwin || linux

package ping

import (
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func Ping(host string, timeoutMs int) (int, error) {
	addr, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", host, err)
	}
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return 0, fmt.Errorf("open unprivileged ICMP socket: %w", err)
	}
	defer conn.Close()

	timeout := time.Duration(timeoutMs) * time.Millisecond
	_ = conn.SetDeadline(time.Now().Add(timeout))
	id := os.Getpid() & 0xffff
	message := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{ID: id, Seq: 1, Data: []byte("sgfg-tui-ping")},
	}
	data, err := message.Marshal(nil)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	if _, err := conn.WriteTo(data, &net.UDPAddr{IP: addr.IP}); err != nil {
		return 0, err
	}
	buffer := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFrom(buffer)
		if err != nil {
			return 0, err
		}
		reply, err := icmp.ParseMessage(1, buffer[:n])
		if err != nil || reply.Type != ipv4.ICMPTypeEchoReply {
			continue
		}
		echo, ok := reply.Body.(*icmp.Echo)
		// Linux ping sockets may rewrite the echo ID; this socket only has one
		// outstanding request, so the sequence number is the stable match key.
		if ok && echo.Seq == 1 {
			return int(time.Since(start).Round(time.Millisecond) / time.Millisecond), nil
		}
	}
}
