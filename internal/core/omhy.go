package core

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"

	omclient "github.com/zjl88858/oh-my-hysteria/core/v2/client"
	"github.com/zjl88858/oh-my-hysteria/extras/v2/obfs"
)

const maxLocalConns = 512

// Track the raw UDP sockets, not the Blink wrapper: QUIC must see Blink's
// QUICPacketSizeLimit contract (1200-byte inner packets, PMTU/GSO disabled).
type blinkConnFactory struct {
	mu      sync.Mutex
	closed  bool
	options obfs.BlinkOptions
	conns   map[*ownedUDPSocket]struct{}
}

type ownedUDPSocket struct {
	*net.UDPConn
	owner *blinkConnFactory
	once  sync.Once
	err   error
}

func (c *ownedUDPSocket) Close() error {
	c.once.Do(func() {
		c.err = c.UDPConn.Close()
		c.owner.mu.Lock()
		delete(c.owner.conns, c)
		c.owner.mu.Unlock()
	})
	return c.err
}

func (f *blinkConnFactory) New(net.Addr) (net.PacketConn, error) {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil, net.ErrClosed
	}
	u, err := net.ListenUDP("udp", nil)
	if err != nil {
		f.mu.Unlock()
		return nil, err
	}
	owned := &ownedUDPSocket{UDPConn: u, owner: f}
	if f.conns == nil {
		f.conns = make(map[*ownedUDPSocket]struct{})
	}
	// One QUIC session per reconnectable client. The upstream reconnect wrapper
	// may drop a closed client's reference before calling Close on its transport;
	// retire its socket rather than retaining it until the user disconnects.
	previous := make([]*ownedUDPSocket, 0, len(f.conns))
	for c := range f.conns {
		previous = append(previous, c)
	}
	f.conns[owned] = struct{}{}
	f.mu.Unlock()
	for _, c := range previous {
		c.Close()
	}
	wrapped, err := obfs.WrapPacketConnBlink(owned, f.options)
	if err != nil {
		owned.Close()
		return nil, err
	}
	return wrapped, nil
}

func (f *blinkConnFactory) Close() {
	f.mu.Lock()
	f.closed = true
	conns := make([]*ownedUDPSocket, 0, len(f.conns))
	for c := range f.conns {
		conns = append(conns, c)
	}
	f.mu.Unlock()
	for _, c := range conns {
		c.Close()
	}
}

type OMHYInstance struct {
	client   omclient.Client
	listener net.Listener
	factory  *blinkConnFactory
	cancel   context.CancelFunc
	once     sync.Once
	mu       sync.Mutex
	closed   bool
	conns    map[net.Conn]struct{}
	wg       sync.WaitGroup
}

func resolveOMHY(ctx context.Context, host string, port int) (*net.UDPAddr, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		return net.UDPAddrFromAddrPort(netip.AddrPortFrom(ip, uint16(port))), nil
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("节点域名没有可用地址")
	}
	return net.UDPAddrFromAddrPort(netip.AddrPortFrom(ips[0], uint16(port))), nil
}

func omhyConfig(ctx context.Context, sel Selection, port int, factory *blinkConnFactory) (*omclient.Config, error) {
	min, max := sel.Transport.portRange()
	if !sel.Transport.Valid() || port < min || port > max {
		return nil, fmt.Errorf("无效的 OMHY 模式或端口")
	}
	addr, err := resolveOMHY(ctx, sel.Node.Host, port)
	if err != nil {
		return nil, err
	}
	return &omclient.Config{
		ServerAddr: addr, Auth: sel.Node.Password, ConnFactory: factory,
		TLSConfig:        omclient.TLSConfig{ServerName: sel.Node.Host},
		CongestionConfig: omclient.CongestionConfig{Type: "copa"},
		// No bandwidth override: do not negotiate Brutal instead of COPA.
	}, nil
}

// The upstream reconnect wrapper serializes calls to this closure. A fresh
// attempt gets a single entry port, never a hopping PacketConn under Blink.
func omhyConfigSource(ctx context.Context, sel Selection, factory *blinkConnFactory) func() (*omclient.Config, error) {
	first := true
	return func() (*omclient.Config, error) {
		port := sel.Port
		if !first {
			min, max := sel.Transport.portRange()
			var err error
			port, err = randPort(min, max)
			if err != nil {
				return nil, err
			}
		}
		first = false
		resolveCtx, stop := context.WithTimeout(ctx, 8*time.Second)
		defer stop()
		return omhyConfig(resolveCtx, sel, port, factory)
	}
}

func StartOMHY(sel Selection, method, listenAddr string, listenPort int, traffic *Traffic, log func(string, string)) (*OMHYInstance, error) {
	if !sel.Transport.Valid() {
		return nil, fmt.Errorf("不支持的 OMHY 模式")
	}
	if method != "http" && method != "socks5" {
		return nil, fmt.Errorf("不支持的本地代理协议")
	}
	min, max := sel.Transport.portRange()
	if sel.Port < min || sel.Port > max {
		return nil, fmt.Errorf("OMHY 端口不在所选模式范围内")
	}
	factory := &blinkConnFactory{options: sel.BlinkConfig().options()}
	// Validate the full upstream options before reporting the listener ready.
	if err := sel.BlinkConfig().validate(); err != nil {
		return nil, fmt.Errorf("Blink 参数无效: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	configFunc := omhyConfigSource(ctx, sel, factory)
	client, err := omclient.NewReconnectableClient(configFunc, func(_ omclient.Client, info *omclient.HandshakeInfo, count int) {
		if log != nil {
			log("info", fmt.Sprintf("OMHY 已连接 #%d: %s · COPA / Blink · UDP=%v", count, info.ServerAddr, info.UDPEnabled))
		}
	}, true)
	if err != nil {
		cancel()
		factory.Close()
		return nil, err
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(listenAddr, strconv.Itoa(listenPort)))
	if err != nil {
		cancel()
		factory.Close()
		client.Close()
		return nil, err
	}
	inst := &OMHYInstance{client: client, listener: ln, factory: factory, cancel: cancel, conns: make(map[net.Conn]struct{})}
	if traffic != nil {
		inst.client = &trafficHyClient{Client: client, traffic: traffic}
	}
	inst.wg.Add(1)
	go inst.acceptLoop(method)
	return inst, nil
}

func (h *OMHYInstance) acceptLoop(method string) {
	defer h.wg.Done()
	for {
		conn, err := h.listener.Accept()
		if err != nil {
			return
		}
		h.mu.Lock()
		if h.closed || len(h.conns) >= maxLocalConns {
			h.mu.Unlock()
			conn.Close()
			continue
		}
		h.conns[conn] = struct{}{}
		h.wg.Add(1)
		h.mu.Unlock()
		go func() {
			defer h.wg.Done()
			defer func() { conn.Close(); h.mu.Lock(); delete(h.conns, conn); h.mu.Unlock() }()
			if method == "socks5" {
				handleSocks5(conn, h.client)
			} else {
				handleHTTP(conn, h.client)
			}
		}()
	}
}

func (h *OMHYInstance) Close() {
	h.once.Do(func() {
		h.cancel()
		h.mu.Lock()
		h.closed = true
		h.listener.Close()
		for c := range h.conns {
			c.Close()
		}
		h.mu.Unlock()
		// Interrupt in-flight QUIC authentication before waiting on the upstream
		// reconnect mutex; Close must not leave a dialing client alive.
		h.factory.Close()
		h.client.Close()
		h.wg.Wait()
	})
}

func (h *OMHYInstance) Addr() string { return h.listener.Addr().String() }
