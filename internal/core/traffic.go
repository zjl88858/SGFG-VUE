package core

import (
	"net"
	"sync"

	hyclient "github.com/zjl88858/oh-my-hysteria/core/v2/client"
)

// Traffic counts application bytes at the remote proxy connection boundary.
// A fresh instance per connection keeps late I/O from an old session isolated.
type Traffic struct {
	mu     sync.Mutex
	totals TrafficSnapshot
	frozen bool
}

type TrafficSnapshot struct {
	Upload   uint64 `json:"upload"`
	Download uint64 `json:"download"`
}

func (t *Traffic) add(upload bool, n int) {
	if t == nil || n <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.frozen {
		return
	}
	if upload {
		t.totals.Upload += uint64(n)
	} else {
		t.totals.Download += uint64(n)
	}
}

func (t *Traffic) Snapshot() TrafficSnapshot {
	if t == nil {
		return TrafficSnapshot{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.totals
}

func (t *Traffic) Freeze() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.frozen = true
	t.mu.Unlock()
}

type trafficConn struct {
	net.Conn
	traffic *Traffic
}

func (c *trafficConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	c.traffic.add(false, n)
	return n, err
}

func (c *trafficConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	c.traffic.add(true, n)
	return n, err
}

type trafficHyClient struct {
	hyclient.Client
	traffic *Traffic
}

func (c *trafficHyClient) TCP(addr string) (net.Conn, error) {
	conn, err := c.Client.TCP(addr)
	if err != nil {
		return nil, err
	}
	return &trafficConn{Conn: conn, traffic: c.traffic}, nil
}

func (c *trafficHyClient) UDP() (hyclient.HyUDPConn, error) {
	conn, err := c.Client.UDP()
	if err != nil {
		return nil, err
	}
	return &trafficHyUDP{HyUDPConn: conn, traffic: c.traffic}, nil
}

type trafficHyUDP struct {
	hyclient.HyUDPConn
	traffic *Traffic
}

func (c *trafficHyUDP) Send(p []byte, addr string) error {
	err := c.HyUDPConn.Send(p, addr)
	if err == nil {
		c.traffic.add(true, len(p))
	}
	return err
}

func (c *trafficHyUDP) Receive() ([]byte, string, error) {
	p, addr, err := c.HyUDPConn.Receive()
	if err == nil {
		c.traffic.add(false, len(p))
	}
	return p, addr, err
}
