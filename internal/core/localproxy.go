package core

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	hyclient "github.com/zjl88858/oh-my-hysteria/core/v2/client"
)

func handleSocks5(conn net.Conn, hy hyclient.Client) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(15 * time.Second))

	buf := make([]byte, 512)
	if _, err := io.ReadFull(conn, buf[:2]); err != nil || buf[0] != 0x05 {
		return
	}
	nMethods := int(buf[1])
	if _, err := io.ReadFull(conn, buf[:nMethods]); err != nil {
		return
	}
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return
	}

	if _, err := io.ReadFull(conn, buf[:4]); err != nil || buf[0] != 0x05 {
		return
	}
	cmd := buf[1]

	target, err := readSocksAddr(conn, buf)
	if err != nil {
		return
	}

	switch cmd {
	case 0x01: // CONNECT
		remote, err := hy.TCP(target)
		if err != nil {
			conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			return
		}
		defer remote.Close()
		if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
			return
		}
		conn.SetDeadline(time.Time{})
		relay(conn, remote)
	case 0x03: // UDP ASSOCIATE
		handleUDPAssociate(conn, hy, target)
	default:
		conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	}
}

func readSocksAddr(r io.Reader, buf []byte) (string, error) {
	atyp := buf[3]
	var host string
	switch atyp {
	case 0x01:
		if _, err := io.ReadFull(r, buf[:4]); err != nil {
			return "", err
		}
		host = net.IP(buf[:4]).String()
	case 0x03:
		if _, err := io.ReadFull(r, buf[:1]); err != nil {
			return "", err
		}
		l := int(buf[0])
		if _, err := io.ReadFull(r, buf[:l]); err != nil {
			return "", err
		}
		host = string(buf[:l])
	case 0x04:
		if _, err := io.ReadFull(r, buf[:16]); err != nil {
			return "", err
		}
		host = net.IP(buf[:16]).String()
	default:
		return "", fmt.Errorf("unsupported atyp %d", atyp)
	}
	if _, err := io.ReadFull(r, buf[:2]); err != nil {
		return "", err
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(buf[:2])))), nil
}

// Bind the association before accepting any datagrams. Never infer a local
// client's port from its first packet: another local process can win that race.
func udpAssociateClient(peer net.Addr, requested string) (netip.AddrPort, error) {
	if peer == nil {
		return netip.AddrPort{}, fmt.Errorf("missing SOCKS TCP peer")
	}
	tcp, err := netip.ParseAddrPort(peer.String())
	if err != nil {
		return netip.AddrPort{}, err
	}
	udp, err := netip.ParseAddrPort(requested)
	if err != nil || udp.Port() == 0 {
		return netip.AddrPort{}, fmt.Errorf("SOCKS UDP requires an IP address and a nonzero source port")
	}
	peerIP := tcp.Addr().Unmap()
	ip := udp.Addr().Unmap()
	if ip.IsUnspecified() {
		ip = peerIP
	}
	if ip != peerIP {
		return netip.AddrPort{}, fmt.Errorf("SOCKS UDP source must match the TCP peer")
	}
	return netip.AddrPortFrom(ip, udp.Port()), nil
}

func handleUDPAssociate(tcpConn net.Conn, hy hyclient.Client, requested string) {
	client, err := udpAssociateClient(tcpConn.RemoteAddr(), requested)
	if err != nil {
		// Policy rejection, not an invitation to claim the session by sending
		// the first UDP packet. TCP CONNECT and existing helper requests are unchanged.
		tcpConn.Write([]byte{0x05, 0x02, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	bindIP := net.ParseIP("127.0.0.1")
	if ta, ok := tcpConn.LocalAddr().(*net.TCPAddr); ok && ta.IP != nil {
		bindIP = ta.IP
	}
	udpLn, err := net.ListenUDP("udp", &net.UDPAddr{IP: bindIP})
	if err != nil {
		tcpConn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer udpLn.Close()

	hyUDP, err := hy.UDP()
	if err != nil {
		tcpConn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer hyUDP.Close()

	local := udpLn.LocalAddr().(*net.UDPAddr)
	reply := buildUDPRelayHeader(local.String())
	reply[0] = 0x05 // Same address encoding; VER, REP=success, RSV.
	tcpConn.SetDeadline(time.Time{})
	if _, err := tcpConn.Write(reply); err != nil {
		return
	}

	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done); udpLn.Close(); hyUDP.Close() }) }

	go func() {
		io.Copy(io.Discard, tcpConn) // association ends when TCP closes
		stop()
	}()

	go func() { // client -> remote
		pkt := make([]byte, 65535)
		for {
			n, from, err := udpLn.ReadFromUDPAddrPort(pkt)
			if err != nil {
				stop()
				return
			}
			from = netip.AddrPortFrom(from.Addr().Unmap(), from.Port())
			if from != client {
				continue
			}
			data := pkt[:n]
			if len(data) < 4 || data[0] != 0 || data[1] != 0 || data[2] != 0 { // RSV and FRAG must be 0
				continue
			}
			addr, hdrLen, err := parseUDPRelayHeader(data)
			if err != nil {
				continue
			}
			if err := hyUDP.Send(data[hdrLen:], addr); err != nil {
				continue
			}
		}
	}()

	go func() { // remote -> client
		for {
			data, addr, err := hyUDP.Receive()
			if err != nil {
				stop()
				return
			}
			hdr := buildUDPRelayHeader(addr)
			if hdr == nil {
				continue
			}
			pkt := append(hdr, data...)
			udpLn.WriteToUDPAddrPort(pkt, client)
		}
	}()

	<-done
}

func parseUDPRelayHeader(data []byte) (string, int, error) {
	if len(data) < 4 {
		return "", 0, fmt.Errorf("short header")
	}
	atyp := data[3]
	var host string
	var off int
	switch atyp {
	case 0x01:
		if len(data) < 4+4+2 {
			return "", 0, fmt.Errorf("short ipv4")
		}
		host = net.IP(data[4:8]).String()
		off = 8
	case 0x03:
		if len(data) < 5 {
			return "", 0, fmt.Errorf("short domain")
		}
		l := int(data[4])
		if len(data) < 5+l+2 {
			return "", 0, fmt.Errorf("short domain")
		}
		host = string(data[5 : 5+l])
		off = 5 + l
	case 0x04:
		if len(data) < 4+16+2 {
			return "", 0, fmt.Errorf("short ipv6")
		}
		host = net.IP(data[4:20]).String()
		off = 20
	default:
		return "", 0, fmt.Errorf("bad atyp")
	}
	port := binary.BigEndian.Uint16(data[off : off+2])
	return net.JoinHostPort(host, strconv.Itoa(int(port))), off + 2, nil
}

func buildUDPRelayHeader(addr string) []byte {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil
	}
	hdr := []byte{0x00, 0x00, 0x00}
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			hdr = append(hdr, 0x01)
			hdr = append(hdr, v4...)
		} else {
			hdr = append(hdr, 0x04)
			hdr = append(hdr, ip.To16()...)
		}
	} else {
		hdr = append(hdr, 0x03, byte(len(host)))
		hdr = append(hdr, host...)
	}
	var p [2]byte
	binary.BigEndian.PutUint16(p[:], uint16(port))
	hdr = append(hdr, p[:]...)
	return hdr
}

func relay(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(a, b)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(b, a)
		done <- struct{}{}
	}()
	<-done
}

func handleHTTP(conn net.Conn, hy hyclient.Client) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	br := bufio.NewReader(io.LimitReader(conn, 64*1024))
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	if req.Method != http.MethodConnect {
		resp := &http.Response{
			StatusCode: http.StatusMethodNotAllowed,
			ProtoMajor: 1, ProtoMinor: 1,
			Body:   io.NopCloser(&emptyReader{}),
			Header: make(http.Header),
		}
		resp.Write(conn)
		return
	}
	target := req.Host
	if _, _, err := net.SplitHostPort(target); err != nil {
		target = net.JoinHostPort(target, "443")
	}
	remote, err := hy.TCP(target)
	if err != nil {
		conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer remote.Close()
	if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}
	conn.SetDeadline(time.Time{})
	relay(conn, remote)
}

type emptyReader struct{}

func (e *emptyReader) Read([]byte) (int, error) { return 0, io.EOF }
