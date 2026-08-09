package core

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	hyclient "github.com/apernet/hysteria/core/v2/client"
	"github.com/apernet/hysteria/extras/v2/obfs"
)

const maxLocalConns = 512

type obfsConnFactory struct {
	password string
}

func (f *obfsConnFactory) New(net.Addr) (net.PacketConn, error) {
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, err
	}
	o, err := obfs.NewSalamanderObfuscator([]byte(f.password))
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return obfs.WrapPacketConn(conn, o), nil
}

type Hy2Instance struct {
	client   hyclient.Client
	listener net.Listener
	once     sync.Once
	conns    atomic.Int32
}

func StartHy2(server string, port int, auth, obfsPassword, method, listenAddr string, listenPort int) (*Hy2Instance, error) {
	serverAddr := fmt.Sprintf("%s:%d", server, port)
	configFunc := func() (*hyclient.Config, error) {
		addr, err := net.ResolveUDPAddr("udp", serverAddr)
		if err != nil {
			return nil, err
		}
		return &hyclient.Config{
			ServerAddr:  addr,
			Auth:        auth,
			ConnFactory: &obfsConnFactory{password: obfsPassword},
			TLSConfig: hyclient.TLSConfig{
				ServerName: server,
			},
		}, nil
	}

	inst := &Hy2Instance{}
	client, err := hyclient.NewReconnectableClient(configFunc, func(hyclient.Client, *hyclient.HandshakeInfo, int) {}, true)
	if err != nil {
		return nil, err
	}
	inst.client = client
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", listenAddr, listenPort))
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("listen %s:%d: %w", listenAddr, listenPort, err)
	}
	inst.listener = ln
	go inst.acceptLoop(method)
	return inst, nil
}

func (h *Hy2Instance) acceptLoop(method string) {
	for {
		conn, err := h.listener.Accept()
		if err != nil {
			return
		}
		if h.conns.Add(1) > maxLocalConns {
			h.conns.Add(-1)
			_ = conn.Close()
			continue
		}
		go func() {
			defer h.conns.Add(-1)
			if method == "socks5" {
				handleSocks5(conn, h.client)
			} else {
				handleHTTP(conn, h.client)
			}
		}()
	}
}

func (h *Hy2Instance) Close() {
	h.once.Do(func() {
		_ = h.listener.Close()
		go func() { _ = h.client.Close() }()
	})
}
