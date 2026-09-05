package sockfilter

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"
)

func socksAddress(ap netip.AddrPort) []byte {
	b := []byte{1}
	if ap.Addr().Is4() {
		v := ap.Addr().As4()
		b = append(b, v[:]...)
	} else {
		b[0] = 4
		v := ap.Addr().As16()
		b = append(b, v[:]...)
	}
	return binary.BigEndian.AppendUint16(b, ap.Port())
}

func readSocksAddress(r io.Reader) (netip.AddrPort, error) {
	var typ [1]byte
	if _, err := io.ReadFull(r, typ[:]); err != nil {
		return netip.AddrPort{}, err
	}
	n := 4
	if typ[0] == 4 {
		n = 16
	} else if typ[0] != 1 {
		return netip.AddrPort{}, fmt.Errorf("SOCKS5 relay must return an IP address (ATYP=%d)", typ[0])
	}
	b := make([]byte, n+2)
	if _, err := io.ReadFull(r, b); err != nil {
		return netip.AddrPort{}, err
	}
	ip, _ := netip.AddrFromSlice(b[:n])
	return netip.AddrPortFrom(ip, binary.BigEndian.Uint16(b[n:])), nil
}

// openSOCKS never falls back to a direct connection. All dials target the
// application's loopback SOCKS listener; original destinations are only data.
func openSOCKS(ctx context.Context, proxy string, cmd byte, target netip.AddrPort) (net.Conn, netip.AddrPort, error) {
	d := net.Dialer{Timeout: 10 * time.Second}
	c, err := d.DialContext(ctx, "tcp4", proxy)
	if err != nil {
		return nil, netip.AddrPort{}, err
	}
	stop := context.AfterFunc(ctx, func() { c.Close() })
	ok := false
	defer func() {
		stop()
		if !ok {
			c.Close()
		}
	}()
	c.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err = c.Write([]byte{5, 1, 0}); err != nil {
		return nil, netip.AddrPort{}, err
	}
	var auth [2]byte
	if _, err = io.ReadFull(c, auth[:]); err != nil {
		return nil, netip.AddrPort{}, err
	}
	if auth != [2]byte{5, 0} {
		return nil, netip.AddrPort{}, fmt.Errorf("SOCKS5 no-auth negotiation rejected")
	}
	if _, err = c.Write(append([]byte{5, cmd, 0}, socksAddress(target)...)); err != nil {
		return nil, netip.AddrPort{}, err
	}
	var reply [3]byte
	if _, err = io.ReadFull(c, reply[:]); err != nil {
		return nil, netip.AddrPort{}, err
	}
	if reply != [3]byte{5, 0, 0} {
		if reply == [3]byte{5, 3, 0} {
			return nil, netip.AddrPort{}, fmt.Errorf("代理节点连接失败（SOCKS5 REP=3：网络不可达）")
		}
		return nil, netip.AddrPort{}, fmt.Errorf("SOCKS5 command %d failed (reply=%v)", cmd, reply)
	}
	bound, err := readSocksAddress(c)
	if err != nil {
		return nil, netip.AddrPort{}, err
	}
	c.SetDeadline(time.Time{})
	ok = true
	return c, bound, nil
}
