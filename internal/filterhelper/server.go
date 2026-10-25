package filterhelper

import (
	"errors"
	"time"

	"sgfg-ng/internal/sockfilter"
)

type filterSession interface{ Close() error }
type startFilter func(int, []string, func(string, string), sockfilter.Options) (filterSession, error)

func serve(conn connection, parentPID uint32, start startFilter) error {
	w := &wire{connection: conn}
	defer conn.Close()
	m, err := w.receive(10 * time.Second)
	if err != nil {
		return err
	}
	if err := validateStart(m); err != nil {
		_ = w.send(message{Type: "error", Text: err.Error()})
		return err
	}
	// Bounded, non-blocking forwarding: driver callbacks must never wait for IPC.
	logs := make(chan message, 128)
	stopLogs := make(chan struct{})
	logsDone := make(chan struct{})
	go func() {
		defer close(logsDone)
		for {
			select {
			case <-stopLogs:
				return
			case log := <-logs:
				if err := w.send(log); err != nil {
					conn.Close()
					return
				}
			}
		}
	}()
	defer func() { close(stopLogs); <-logsDone }()
	log := func(level, text string) {
		if len(text) > 4096 {
			text = text[:4096]
		}
		select {
		case logs <- message{Type: "log", Level: level, Text: text}:
		default:
		}
	}
	session, err := start(m.Port, m.Names, log, sockfilter.Options{TCPOnly: m.TCPOnly, BypassPID: parentPID})
	if err != nil {
		_ = w.send(message{Type: "error", Text: err.Error()})
		return err
	}
	// Close exactly once, even if startup succeeds just as the parent exits.
	closed := false
	defer func() {
		if !closed {
			_ = session.Close()
		}
	}()
	if err := w.send(message{Type: "ready"}); err != nil {
		return err
	}
	for {
		m, err := w.receive(15 * time.Second)
		if err != nil {
			return err
		}
		switch m.Type {
		case "ping":
			if err := w.send(message{Type: "pong"}); err != nil {
				return err
			}
		case "stop":
			err := session.Close()
			closed = true
			response := message{Type: "stopped"}
			if err != nil {
				response.Text = err.Error()
			}
			return errors.Join(err, w.send(response))
		default:
			return errors.New("helper 只接受心跳和停止请求")
		}
	}
}
