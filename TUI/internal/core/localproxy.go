package core

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	hyclient "github.com/apernet/hysteria/core/v2/client"
)

func handleSocks5(conn net.Conn, hy hyclient.Client) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))

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
	case 0x01:
		remote, err := hy.TCP(target)
		if err != nil {
			_, _ = conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			return
		}
		defer remote.Close()
		if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
			return
		}
		_ = conn.SetDeadline(time.Time{})
		relay(conn, remote)
	case 0x03:
		handleUDPAssociate(conn, hy)
	default:
		_, _ = conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
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

func handleUDPAssociate(tcpConn net.Conn, hy hyclient.Client) {
	bindIP := net.ParseIP("127.0.0.1")
	if ta, ok := tcpConn.LocalAddr().(*net.TCPAddr); ok && ta.IP != nil {
		bindIP = ta.IP
	}
	udpLn, err := net.ListenUDP("udp", &net.UDPAddr{IP: bindIP})
	if err != nil {
		_, _ = tcpConn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer udpLn.Close()

	hyUDP, err := hy.UDP()
	if err != nil {
		_, _ = tcpConn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer hyUDP.Close()

	local := udpLn.LocalAddr().(*net.UDPAddr)
	reply := []byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	copy(reply[4:8], local.IP.To4())
	binary.BigEndian.PutUint16(reply[8:10], uint16(local.Port))
	_ = tcpConn.SetDeadline(time.Time{})
	if _, err := tcpConn.Write(reply); err != nil {
		return
	}

	done := make(chan struct{})
	var once sync.Once
	stop := func() {
		once.Do(func() {
			close(done)
			_ = udpLn.Close()
			_ = hyUDP.Close()
		})
	}
	var addrMu sync.RWMutex
	var clientAddr *net.UDPAddr

	go func() {
		_, _ = io.Copy(io.Discard, tcpConn)
		stop()
	}()
	go func() {
		pkt := make([]byte, 65535)
		for {
			n, from, err := udpLn.ReadFromUDP(pkt)
			if err != nil {
				stop()
				return
			}
			addrMu.Lock()
			clientAddr = from
			addrMu.Unlock()
			data := pkt[:n]
			if len(data) < 4 || data[2] != 0 {
				continue
			}
			addr, headerLen, err := parseUDPRelayHeader(data)
			if err == nil {
				_ = hyUDP.Send(data[headerLen:], addr)
			}
		}
	}()
	go func() {
		for {
			data, addr, err := hyUDP.Receive()
			if err != nil {
				stop()
				return
			}
			header := buildUDPRelayHeader(addr)
			if header == nil {
				continue
			}
			addrMu.RLock()
			to := clientAddr
			addrMu.RUnlock()
			if to != nil {
				_, _ = udpLn.WriteToUDP(append(header, data...), to)
			}
		}
	}()
	<-done
}

func parseUDPRelayHeader(data []byte) (string, int, error) {
	if len(data) < 4 {
		return "", 0, fmt.Errorf("short header")
	}
	var host string
	var off int
	switch data[3] {
	case 0x01:
		if len(data) < 10 {
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
		if len(data) < 22 {
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
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return nil
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 0 || port > 65535 {
		return nil
	}
	header := []byte{0, 0, 0}
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			header = append(header, 0x01)
			header = append(header, v4...)
		} else {
			header = append(header, 0x04)
			header = append(header, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return nil
		}
		header = append(header, 0x03, byte(len(host)))
		header = append(header, host...)
	}
	var p [2]byte
	binary.BigEndian.PutUint16(p[:], uint16(port))
	return append(header, p[:]...)
}

func relay(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(a, b)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(b, a)
		done <- struct{}{}
	}()
	<-done
}

func handleHTTP(conn net.Conn, hy hyclient.Client) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	reader := bufio.NewReader(io.LimitReader(conn, 64*1024))
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}
	if req.Method != http.MethodConnect {
		resp := &http.Response{
			StatusCode: http.StatusMethodNotAllowed,
			ProtoMajor: 1,
			ProtoMinor: 1,
			Body:       io.NopCloser(&emptyReader{}),
			Header:     make(http.Header),
		}
		_ = resp.Write(conn)
		return
	}
	target := req.Host
	if _, _, err := net.SplitHostPort(target); err != nil {
		target = net.JoinHostPort(target, "443")
	}
	remote, err := hy.TCP(target)
	if err != nil {
		_, _ = conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer remote.Close()
	if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}
	_ = conn.SetDeadline(time.Time{})
	relay(conn, remote)
}

type emptyReader struct{}

func (*emptyReader) Read([]byte) (int, error) { return 0, io.EOF }
