//go:build windows && amd64

package sockfilter

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type tcpTarget struct {
	id     uintptr
	remote netip.AddrPort
}

type Session struct {
	api            *sdk
	ctx            context.Context
	cancel         context.CancelFunc
	proxy          string
	log            func(string, string)
	receiveUDP     func(uintptr, address, []byte)
	listeners      map[uint16]*net.TCPListener
	mu             sync.Mutex
	closing        bool
	targets        map[netip.AddrPort]tcpTarget
	tcpIDs         map[uintptr]netip.AddrPort
	conns          map[net.Conn]bool
	udp            map[uintptr]*udpFlow
	queuedUDPBytes int
	wg             sync.WaitGroup
	once           sync.Once
	tcpRequests    atomic.Uint64
	tcpAccepted    atomic.Uint64
	tcpSOCKS       atomic.Uint64
	udpPackets     atomic.Uint64
	udpSOCKS       atomic.Uint64
	udpReceived    atomic.Uint64
	tcpOnly        bool
	udpDropped     atomic.Uint64
	dns            map[uintptr]*dnsEndpoint
	dnsPending     int
	dnsQueries     atomic.Uint64
	dnsReplies     atomic.Uint64
	dnsFailures    atomic.Uint64
	driver         *driverLease
	closeErr       error
	debug          bool
	tcpFailures    atomic.Uint64
	bypassPID      uint32
}

func equalProcessName(a, b string) bool { return strings.EqualFold(a, b) }

func Start(port int, names []string, log func(string, string), options ...Options) (_ *Session, err error) {
	names, err = NormalizeNames(names)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("启用进程代理前请至少指定一个 .exe 文件名")
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("SOCKS5 端口无效")
	}
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if active.Load() != nil {
		return nil, fmt.Errorf("SockFilter 已在使用中")
	}
	if log == nil {
		log = func(string, string) {}
	}
	if proxifierRunning() {
		log("warn", "检测到其他代理软件，请避免对同一进程重复代理")
	}
	a, err := loadSDK()
	if err != nil {
		return nil, err
	}
	lease, err := acquireDriver()
	if err != nil {
		a.dll.Release()
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, lease.Close())
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{api: a, ctx: ctx, cancel: cancel, proxy: fmt.Sprintf("127.0.0.1:%d", port), log: log,
		driver: lease, debug: os.Getenv("SGFG_SOCKFILTER_DEBUG") == "1",
		receiveUDP: a.receiveUDP,
		listeners:  make(map[uint16]*net.TCPListener), targets: make(map[netip.AddrPort]tcpTarget), tcpIDs: make(map[uintptr]netip.AddrPort),
		conns: make(map[net.Conn]bool), udp: make(map[uintptr]*udpFlow)}
	if len(options) > 0 {
		s.tcpOnly = options[0].TCPOnly
		s.bypassPID = options[0].BypassPID
	}
	for _, family := range []uint16{2, 23} {
		network, addr := "tcp4", "0.0.0.0:0"
		if family == 23 {
			network, addr = "tcp6", "[::]:0"
		}
		la, _ := net.ResolveTCPAddr(network, addr)
		ln, err := net.ListenTCP(network, la)
		if err != nil {
			for _, l := range s.listeners {
				l.Close()
			}
			cancel()
			a.dll.Release()
			return nil, fmt.Errorf("启动 SockFilter IPv%d 重定向监听失败: %w", map[uint16]int{2: 4, 23: 6}[family], err)
		}
		s.listeners[family] = ln
	}
	initCallbacks()
	active.Store(s)
	// SCM lifecycle is managed explicitly above, not implicitly by the SDK.
	a.call("nf_setOptions", 1, 4|8)
	name := append([]byte(managedDriverName), 0)
	r, _, winErr := a.procs["nf_init"].Call(uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(&callbacks[0])))
	runtime.KeepAlive(name)
	if int32(r) != 0 {
		active.Store(nil)
		for _, l := range s.listeners {
			l.Close()
		}
		cancel()
		a.dll.Release()
		return nil, fmt.Errorf("连接进程驱动失败 (NF_STATUS=%d, Windows=%v)，请确认驱动未被其他程序占用", int32(r), winErr)
	}
	if a.call("nf_getDriverType") != 3 {
		s.Close()
		return nil, fmt.Errorf("已连接的驱动不是 SockFilter (DT_SOCK)，已停止")
	}
	for family, ln := range s.listeners {
		s.wg.Add(1)
		go s.accept(family, ln)
	}
	rules := makeRules(names, uint32(os.Getpid()), s.bypassPID)
	r = a.call("nf_setRulesEx", uintptr(unsafe.Pointer(&rules[0])), uintptr(len(rules)))
	runtime.KeepAlive(rules)
	if int32(r) != 0 {
		s.Close()
		return nil, fmt.Errorf("写入 SockFilter 规则失败 (NF_STATUS=%d)，已回滚", int32(r))
	}
	log("info", "进程代理已开启: "+strings.Join(names, ", "))
	if s.tcpOnly {
		log("info", "当前上游仅支持 TCP，DNS 查询自动转换为 TCP")
	}
	if s.debug {
		s.wg.Add(1)
		go s.reportStats()
	}
	return s, nil
}

func (s *Session) tcpConnect(id uintptr, info *[67]byte) {
	n := s.tcpRequests.Add(1)
	remote := address(info[39:67])
	ap, err := remote.addrPort()
	if err != nil {
		binary.LittleEndian.PutUint32(info[:4], 1)
		return
	}
	if s.debug && n <= 5 {
		s.log("info", fmt.Sprintf("SockFilter TCP 命中: PID=%d → %s", binary.LittleEndian.Uint32(info[4:8]), ap))
	}
	if ap.Port() != 53 && bypass(ap) {
		binary.LittleEndian.PutUint32(info[:4], 0)
		return
	}
	pid := binary.LittleEndian.Uint32(info[4:8])
	// Do not call nf_tcpIsProxy here: the supplied SockFilter DLL returned
	// 0xffffffff for ordinary clients, so a BOOL-style != 0 check bypassed
	// every matched connection. The SDK sample relies on its built-in redirect
	// protection plus excluding ourselves/loopback, which we retain below.
	if pid == uint32(os.Getpid()) || (s.bypassPID != 0 && pid == s.bypassPID) {
		binary.LittleEndian.PutUint32(info[:4], 0)
		return
	}
	local := address(info[11:39])
	lap, err := local.addrPort()
	family := binary.LittleEndian.Uint16(info[9:11])
	ln := s.listeners[family]
	if err != nil || ln == nil || lap.Port() == 0 {
		binary.LittleEndian.PutUint32(info[:4], 1)
		return
	}
	ip := lap.Addr()
	if ip.IsUnspecified() {
		if family == 2 {
			ip = netip.MustParseAddr("127.0.0.1")
		} else {
			ip = netip.IPv6Loopback()
		}
	}
	key := netip.AddrPortFrom(ip, lap.Port())
	s.mu.Lock()
	_, collision := s.targets[key]
	if s.closing || collision || len(s.tcpIDs) >= 4096 {
		s.mu.Unlock()
		binary.LittleEndian.PutUint32(info[:4], 1)
		return
	}
	s.targets[key] = tcpTarget{id, tcpDestination(ap)}
	s.tcpIDs[id] = key
	s.mu.Unlock()
	// Same approach as the SDK SocksRedirector: redirect to the bound local
	// address (or loopback), retaining the original destination in a table.
	to := fromAddrPort(netip.AddrPortFrom(ip, uint16(ln.Addr().(*net.TCPAddr).Port)))
	copy(info[39:67], to[:])
	binary.LittleEndian.PutUint32(info[4:8], uint32(os.Getpid()))
}

func (s *Session) tcpClosed(id uintptr) {
	s.mu.Lock()
	if key, ok := s.tcpIDs[id]; ok {
		if t, exists := s.targets[key]; exists && t.id == id {
			delete(s.targets, key)
		}
		delete(s.tcpIDs, id)
	}
	s.mu.Unlock()
}

func (s *Session) accept(family uint16, ln *net.TCPListener) {
	defer s.wg.Done()
	for {
		c, err := ln.AcceptTCP()
		if err != nil {
			return
		}
		key := c.RemoteAddr().(*net.TCPAddr).AddrPort()
		// tcp6 keeps IPv6, tcp4 always uses canonical IPv4 keys.
		if family == 2 {
			key = netip.AddrPortFrom(key.Addr().Unmap(), key.Port())
		}
		s.mu.Lock()
		t, ok := s.targets[key]
		if ok {
			delete(s.targets, key)
		}
		if !ok || s.closing {
			s.mu.Unlock()
			c.Close()
			continue
		}
		s.conns[c] = true
		s.tcpAccepted.Add(1)
		s.wg.Add(1)
		s.mu.Unlock()
		go s.relayTCP(c, t.remote)
	}
}

func (s *Session) relayTCP(c *net.TCPConn, target netip.AddrPort) {
	defer s.wg.Done()
	defer func() { c.Close(); s.mu.Lock(); delete(s.conns, c); s.mu.Unlock() }()
	up, _, err := openSOCKS(s.ctx, s.proxy, 1, target)
	if err != nil {
		if s.ctx.Err() == nil {
			if n := s.tcpFailures.Add(1); s.debug || n <= 5 {
				s.log("warn", fmt.Sprintf("进程代理连接 %s 失败: %v", target, err))
			}
		}
		return
	}
	defer up.Close()
	if n := s.tcpSOCKS.Add(1); s.debug && n <= 5 {
		s.log("info", fmt.Sprintf("SockFilter SOCKS5 CONNECT 成功: %s", target))
	}
	stop := context.AfterFunc(s.ctx, func() { up.Close() })
	defer stop()
	done := make(chan struct{})
	go func() {
		io.Copy(up, c)
		if t, ok := up.(*net.TCPConn); ok {
			t.CloseWrite()
		}
		close(done)
	}()
	io.Copy(c, up)
	c.Close()
	up.Close()
	<-done
}

// Close stops producers and joins all network workers before nf_free / DLL
// unload. Callbacks stay installed while nf_free drains the SDK threads.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		defer func() {
			if s.debug {
				s.log("info", s.stats())
			}
		}()
		s.mu.Lock()
		s.closing = true
		s.cancel()
		for _, ln := range s.listeners {
			ln.Close()
		}
		for c := range s.conns {
			c.Close()
		}
		for _, f := range s.udp {
			f.cancel()
		}
		s.mu.Unlock()
		s.api.call("nf_deleteRules")
		s.wg.Wait()
		s.api.call("nf_free")
		active.CompareAndSwap(s, nil)
		s.api.dll.Release()
		s.closeErr = s.driver.Close()
	})
	return s.closeErr
}

type udpPacket struct {
	addr netip.AddrPort
	data []byte
}
type udpFlow struct {
	ctx         context.Context
	cancel      context.CancelFunc
	queue       chan udpPacket
	family      uint16
	queuedBytes int // guarded by Session.mu
}

func (s *Session) udpClosed(id uintptr) {
	s.mu.Lock()
	if f := s.dns[id]; f != nil {
		f.cancel()
		delete(s.dns, id)
	}
	if f := s.udp[id]; f != nil {
		f.cancel()
		delete(s.udp, id)
	}
	s.mu.Unlock()
}

func (s *Session) udpSend(id uintptr, addr *address, data unsafe.Pointer, size uintptr, opts unsafe.Pointer) {
	ap, err := addr.addrPort()
	if err != nil || size > 65507 {
		return
	}
	// DNS comes before local-address bypass: a selected app may use a router
	// or loopback DNS stub. The response still appears from that original peer.
	if ap.Port() == 53 {
		s.dnsSend(id, *addr, data, size)
		return
	}
	if bypass(ap) {
		s.api.call("nf_udpPostSend", id, uintptr(unsafe.Pointer(addr)), uintptr(data), size, uintptr(opts))
		return
	}
	if s.tcpOnly {
		if s.udpDropped.Add(1) == 1 {
			s.log("info", "当前传输不支持 UDP，该流量未转发")
		}
		return
	}
	if n := s.udpPackets.Add(1); s.debug && n <= 5 {
		s.log("info", fmt.Sprintf("SockFilter UDP 命中: endpoint=%d → %s", id, ap))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return
	}
	f := s.udp[id]
	if f == nil {
		if len(s.udp) >= 512 {
			return
		} // Bound both goroutines and queue memory.
		ctx, cancel := context.WithCancel(s.ctx)
		f = &udpFlow{ctx: ctx, cancel: cancel, queue: make(chan udpPacket, 32), family: binary.LittleEndian.Uint16(addr[:2])}
		s.udp[id] = f
		s.wg.Add(1)
		go s.runUDP(id, f)
	}
	if f.ctx.Err() != nil || s.queuedUDPBytes+int(size) > 16*1024*1024 {
		return
	}
	// SDK-owned buffers are only valid until this callback returns.
	p := udpPacket{ap, append([]byte(nil), unsafe.Slice((*byte)(data), int(size))...)}
	select {
	case f.queue <- p:
		f.queuedBytes += len(p.data)
		s.queuedUDPBytes += len(p.data)
	default:
	} // Drop on overload, never leak via direct send.
}

func (s *Session) runUDP(id uintptr, f *udpFlow) {
	defer s.wg.Done()
	defer func() {
		f.cancel()
		s.mu.Lock()
		s.queuedUDPBytes -= f.queuedBytes
		f.queuedBytes = 0
		// Failed flows stay in the map, but must not retain queued payloads.
		for len(f.queue) > 0 {
			<-f.queue
		}
		s.mu.Unlock()
	}()
	// Keep a failed flow in the map until udpClosed: subsequent packets are
	// dropped rather than repeatedly reconnecting or silently going direct.
	u, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return
	}
	defer u.Close()
	stopUDP := context.AfterFunc(f.ctx, func() { u.Close() })
	defer stopUDP()
	local := u.LocalAddr().(*net.UDPAddr).AddrPort()
	ctrl, bound, err := openSOCKS(f.ctx, s.proxy, 3, netip.AddrPortFrom(local.Addr().Unmap(), local.Port()))
	if err != nil {
		if f.ctx.Err() == nil {
			s.log("warn", fmt.Sprintf("UDP 转发连接失败: %v", err))
		}
		return
	}
	defer ctrl.Close()
	stopTCP := context.AfterFunc(f.ctx, func() { ctrl.Close() })
	defer stopTCP()
	if bound.Addr().IsUnspecified() {
		bound = netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), bound.Port())
	}
	bound = netip.AddrPortFrom(bound.Addr().Unmap(), bound.Port())
	if !bound.Addr().Is4() || !bound.Addr().IsLoopback() || bound.Port() == 0 {
		s.log("warn", "SockFilter 拒绝非 IPv4 loopback SOCKS5 UDP 中继地址")
		return
	}
	if n := s.udpSOCKS.Add(1); s.debug && n <= 5 {
		s.log("info", fmt.Sprintf("SockFilter UDP ASSOCIATE 成功: endpoint=%d", id))
	}
	var workers sync.WaitGroup
	workers.Add(2)
	defer func() { f.cancel(); u.Close(); ctrl.Close(); workers.Wait() }()
	go func() { defer workers.Done(); io.Copy(io.Discard, ctrl); f.cancel() }()
	go func() {
		defer workers.Done()
		buf := make([]byte, 65535)
		for {
			n, from, err := u.ReadFromUDPAddrPort(buf)
			if err != nil {
				f.cancel()
				return
			}
			from = netip.AddrPortFrom(from.Addr().Unmap(), from.Port())
			if from != bound || n < 4 || buf[0] != 0 || buf[1] != 0 || buf[2] != 0 {
				continue
			}
			reader := &packetReader{b: buf[3:n]}
			remote, err := readSocksAddress(reader)
			if err != nil {
				continue
			}
			if f.family == 23 && remote.Addr().Is4() {
				remote = netip.AddrPortFrom(netip.AddrFrom16(remote.Addr().As16()), remote.Port())
			} else if f.family == 2 {
				remote = netip.AddrPortFrom(remote.Addr().Unmap(), remote.Port())
				if !remote.Addr().Is4() {
					continue
				}
			}
			// SDK can synthesize default NF_UDP_OPTIONS for receive. Do not
			// retain send-side ancillary data (possibly a different destination).
			a := fromAddrPort(remote)
			payload := reader.b
			s.receiveUDP(id, a, payload)
			s.udpReceived.Add(1)
		}
	}()
	idle := time.NewTimer(2 * time.Minute)
	defer idle.Stop()
	for {
		select {
		case <-f.ctx.Done():
			return
		case <-idle.C:
			// Allow a long-lived UDP socket to create a new association later.
			s.mu.Lock()
			if s.udp[id] == f {
				delete(s.udp, id)
			}
			s.mu.Unlock()
			return
		case p := <-f.queue:
			s.mu.Lock()
			f.queuedBytes -= len(p.data)
			s.queuedUDPBytes -= len(p.data)
			s.mu.Unlock()
			idle.Reset(2 * time.Minute)
			b := append([]byte{0, 0, 0}, socksAddress(p.addr)...)
			b = append(b, p.data...)
			if _, err := u.WriteToUDPAddrPort(b, bound); err != nil {
				return
			}
		}
	}
}

type packetReader struct{ b []byte }

func (s *Session) stats() string {
	return fmt.Sprintf("SockFilter 统计: TCP命中=%d 接入=%d SOCKS成功=%d；UDP发送=%d 关联=%d 回包=%d 丢弃=%d；DNS转TCP=%d 回包=%d 失败=%d", s.tcpRequests.Load(), s.tcpAccepted.Load(), s.tcpSOCKS.Load(), s.udpPackets.Load(), s.udpSOCKS.Load(), s.udpReceived.Load(), s.udpDropped.Load(), s.dnsQueries.Load(), s.dnsReplies.Load(), s.dnsFailures.Load())
}

func (s *Session) reportStats() {
	defer s.wg.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	previous := ""
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			now := s.stats()
			if now == previous {
				continue
			}
			if s.tcpRequests.Load() == 0 && s.udpPackets.Load() == 0 && s.dnsQueries.Load() == 0 && s.udpDropped.Load() == 0 {
				s.log("warn", "SockFilter 尚无规则命中：请确认列表已保存、实际联网进程名正确，并重启目标进程；规则下发成功不等于流量已代理。")
			} else {
				s.log("info", now)
			}
			previous = now
		}
	}
}

func (r *packetReader) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.b)
	r.b = r.b[n:]
	return n, nil
}
