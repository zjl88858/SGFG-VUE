package main

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"sgfg-ng/internal/core"
	"sgfg-ng/internal/proxy"
)

type ProxyService struct {
	mgr    *proxy.Manager
	app    *application.App
	mu     sync.Mutex
	optWin *application.WebviewWindow
}

const Version = "v11.4"

func NewProxyService(mgr *proxy.Manager) *ProxyService {
	return &ProxyService{mgr: mgr}
}

func (s *ProxyService) attachApp(app *application.App) {
	s.app = app
}

func (s *ProxyService) Version() string {
	return Version
}

func (s *ProxyService) SelectTransport(transport string) proxy.SelectResult {
	return s.mgr.SelectTransport(transport)
}

func (s *ProxyService) PingSelected() proxy.PingResult {
	return s.mgr.PingSelected()
}

func (s *ProxyService) SetOptions(opts proxy.ConnectOptions) string {
	if err := s.mgr.SetOptions(opts); err != nil {
		return err.Error()
	}
	return ""
}

func (s *ProxyService) Options() proxy.ConnectOptions {
	return s.mgr.Options()
}

func (s *ProxyService) RelayOptions() proxy.RelayOptions {
	return s.mgr.RelayOptions()
}

func (s *ProxyService) SetRelayOptions(opts proxy.RelayOptions) string {
	if err := s.mgr.SetRelayOptions(opts); err != nil {
		return err.Error()
	}
	return ""
}

func (s *ProxyService) SettingsError() string { return s.mgr.SettingsError() }

func (s *ProxyService) Traffic() core.TrafficSnapshot { return s.mgr.Traffic() }

func (s *ProxyService) Connect() string {
	if err := s.mgr.Connect(); err != nil {
		return err.Error()
	}
	return ""
}

func (s *ProxyService) Disconnect() string {
	if err := s.mgr.Disconnect(); err != nil {
		return err.Error()
	}
	return ""
}

func (s *ProxyService) StartRelay(port int, password string) string {
	if err := s.mgr.StartRelay(port, password); err != nil {
		return err.Error()
	}
	return ""
}

func (s *ProxyService) StopRelay() string {
	if err := s.mgr.StopRelay(); err != nil {
		return err.Error()
	}
	return ""
}

func (s *ProxyService) GeneratePassword() string {
	return core.GeneratePassword()
}

func (s *ProxyService) Status() proxy.StatusInfo {
	return s.mgr.Status()
}

func (s *ProxyService) OpenOptions() {
	s.mu.Lock()
	w := s.optWin
	s.mu.Unlock()
	if w != nil {
		w.Focus()
		return
	}

	win := s.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "options",
		Title:            "选项 - SGFG-VUE",
		Width:            470,
		Height:           545,
		MinWidth:         430,
		MinHeight:        540,
		URL:              "/#/options",
		Frameless:        true,
		BackgroundColour: application.NewRGB(0xe8, 0xfa, 0xec),
	})

	s.mu.Lock()
	s.optWin = win
	s.mu.Unlock()

	win.OnWindowEvent(events.Common.WindowClosing, func(e *application.WindowEvent) {
		s.mu.Lock()
		s.optWin = nil
		s.mu.Unlock()
	})
}

func (s *ProxyService) CloseOptions() {
	s.mu.Lock()
	w := s.optWin
	s.mu.Unlock()
	if w != nil {
		w.Close()
	}
}
