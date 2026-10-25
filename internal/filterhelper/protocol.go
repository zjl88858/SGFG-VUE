// Package filterhelper isolates privileged process filtering from the GUI.
package filterhelper

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"sgfg-ng/internal/sockfilter"
)

const helperName = "sgfg-driver-helper.exe"
const protocolVersion = 1
const maxMessage = 64 * 1024
const startupTimeout = 45 * time.Second
const cleanupTimeout = 35 * time.Second

type message struct {
	Version int      `json:"version"`
	Type    string   `json:"type"`
	Port    int      `json:"port,omitempty"`
	Names   []string `json:"names,omitempty"`
	TCPOnly bool     `json:"tcpOnly,omitempty"`
	Level   string   `json:"level,omitempty"`
	Text    string   `json:"text,omitempty"`
}

type connection interface {
	io.ReadWriteCloser
	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
}

type wire struct {
	connection
	writeMu sync.Mutex
}

func (w *wire) send(m message) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	m.Version = protocolVersion
	p, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if len(p) > maxMessage {
		return errors.New("helper 消息超出大小限制")
	}
	if err := w.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], uint32(len(p)))
	return writeAll(w.connection, append(size[:], p...))
}

func writeAll(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}

func (w *wire) receive(timeout time.Duration) (message, error) {
	var m message
	if err := w.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return m, err
	}
	var size [4]byte
	if _, err := io.ReadFull(w.connection, size[:]); err != nil {
		return m, err
	}
	n := binary.LittleEndian.Uint32(size[:])
	if n == 0 || n > maxMessage {
		return m, errors.New("无效的 helper 消息长度")
	}
	p := make([]byte, n)
	if _, err := io.ReadFull(w.connection, p); err != nil {
		return m, err
	}
	d := json.NewDecoder(bytes.NewReader(p))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return m, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return m, errors.New("无效的 helper 消息内容")
	}
	if m.Version != protocolVersion {
		return m, errors.New("驱动 helper 版本不匹配，请使用同一版本的完整程序目录")
	}
	return m, nil
}

func validateStart(m message) error {
	if m.Type != "start" || m.Port < 1 || m.Port > 65535 {
		return errors.New("无效的进程代理启动请求")
	}
	names, err := sockfilter.NormalizeNames(m.Names)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("进程代理列表不能为空")
	}
	return nil
}
