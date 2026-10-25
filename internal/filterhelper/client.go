package filterhelper

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type Client struct {
	w           *wire
	done        chan struct{}
	mu          sync.Mutex
	err         error
	once        sync.Once
	closeErr    error
	processDone <-chan struct{}
}

func (c *Client) Done() <-chan struct{} { return c.done }

func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func startClient(conn connection, request message, log func(string, string)) (*Client, error) {
	w := &wire{connection: conn}
	if err := validateStart(request); err != nil {
		conn.Close()
		return nil, err
	}
	if err := w.send(request); err != nil {
		conn.Close()
		return nil, err
	}
	// One absolute limit: log messages cannot extend startup indefinitely.
	deadline := time.Now().Add(startupTimeout)
	for {
		m, err := w.receive(time.Until(deadline))
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("等待驱动 helper 就绪失败: %w", err)
		}
		switch m.Type {
		case "log":
			if log != nil {
				log(m.Level, m.Text)
			}
		case "error":
			conn.Close()
			return nil, errors.New(m.Text)
		case "ready":
			c := &Client{w: w, done: make(chan struct{})}
			go c.readLoop(log)
			go c.heartbeat()
			return c, nil
		default:
			conn.Close()
			return nil, errors.New("无效的 helper 启动响应")
		}
	}
}

func (c *Client) readLoop(log func(string, string)) {
	defer close(c.done)
	defer c.w.Close()
	for {
		m, err := c.w.receive(startupTimeout)
		if err == nil {
			switch m.Type {
			case "log":
				if log != nil {
					log(m.Level, m.Text)
				}
				continue
			case "pong":
				continue
			case "stopped":
				c.mu.Lock()
				if m.Text != "" {
					c.err = errors.New(m.Text)
				}
				c.mu.Unlock()
				return
			default:
				err = errors.New("无效的 helper 响应")
			}
		}
		c.mu.Lock()
		c.err = fmt.Errorf("驱动 helper 通信中断: %w", err)
		c.mu.Unlock()
		return
	}
}

func (c *Client) heartbeat() {
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-t.C:
			if err := c.w.send(message{Type: "ping"}); err != nil {
				c.w.Close()
				return
			}
		}
	}
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	c.once.Do(func() {
		select {
		case <-c.done:
			c.closeErr = c.Err()
			return
		default:
		}
		if err := c.w.send(message{Type: "stop"}); err != nil {
			c.w.Close()
		}
		t := time.NewTimer(cleanupTimeout)
		defer t.Stop()
		select {
		case <-c.done:
			c.closeErr = c.Err()
		case <-t.C:
			c.w.Close()
			c.closeErr = errors.New("等待驱动 helper 清理超时，请确认进程代理驱动状态")
		}
	})
	if c.processDone != nil {
		select {
		case <-c.processDone:
		case <-time.After(5 * time.Second):
			return errors.Join(c.closeErr, errors.New("驱动 helper 尚未退出，请确认驱动清理状态"))
		}
	}
	return c.closeErr
}
