package proxy

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"sgfg-tui/internal/core"
	"sgfg-tui/internal/ping"
	"sgfg-tui/internal/sysproxy"
)

type State int

const (
	StateIdle State = iota
	StateConnecting
	StateConnected
	StateDisconnecting
)

type StatusInfo struct {
	State       string
	Transport   string
	NodeLabel   string
	NodeName    string
	Host        string
	Port        int
	LocalPort   int
	Method      string
	ListenAddr  string
	SysProxy    bool
	RelayActive bool
	RelayPort   int
}

type SelectResult struct {
	NodeLabel string
	NodeName  string
	Host      string
	Port      int
	Latency   int
	Reachable bool
}

type PingResult struct {
	Latency   int
	Reachable bool
	Err       error
}

type ConnectOptions struct {
	UseSocks5 bool
	AllowLAN  bool
	SysProxy  bool
	LocalPort int
}

type LogEvent struct {
	Level   string
	Message string
}

type ScanEvent struct {
	Attempt int
	Max     int
	Host    string
}

type Manager struct {
	mu          sync.Mutex
	state       State
	sel         *core.Selection
	opts        ConnectOptions
	proxy       any
	localPort   int
	relay       *core.GostInstance
	relayOn     bool
	relayPort   int
	sysProxyOn  bool
	systemProxy sysproxy.Controller
	emit        func(any)
	shutdown    bool
}

func NewManager(systemProxy sysproxy.Controller) *Manager {
	available, _ := systemProxy.Available()
	return &Manager{
		state:       StateIdle,
		opts:        ConnectOptions{SysProxy: available},
		systemProxy: systemProxy,
	}
}

func (m *Manager) SetEmitter(emit func(any)) {
	m.mu.Lock()
	m.emit = emit
	m.mu.Unlock()
}

func (m *Manager) send(event any) {
	m.mu.Lock()
	emit := m.emit
	m.mu.Unlock()
	if emit != nil {
		emit(event)
	}
}

func (m *Manager) log(level, message string) {
	m.send(LogEvent{Level: level, Message: message})
}

func (m *Manager) SystemProxyCapability() (bool, string) {
	return m.systemProxy.Available()
}

func (m *Manager) SetOptions(opts ConnectOptions) error {
	if opts.LocalPort != 0 && (opts.LocalPort < 1024 || opts.LocalPort > 65535) {
		return fmt.Errorf("端口需在 1024-65535 之间，或为 0（默认 %d）", core.LocalProxyPort)
	}
	if opts.UseSocks5 {
		opts.SysProxy = false
	}
	if opts.SysProxy {
		if available, reason := m.systemProxy.Available(); !available {
			return fmt.Errorf("自动系统代理不可用: %s", reason)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != StateIdle {
		return errors.New("连接期间无法修改代理选项")
	}
	m.opts = opts
	return nil
}

func (m *Manager) Options() ConnectOptions {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.opts
}

func (m *Manager) SelectTransport(transport string) (SelectResult, error) {
	m.mu.Lock()
	if m.shutdown {
		m.mu.Unlock()
		return SelectResult{}, errors.New("程序正在退出")
	}
	if m.state != StateIdle {
		m.mu.Unlock()
		return SelectResult{}, errors.New("已连接状态下无法切换传输")
	}
	previous := ""
	if m.sel != nil {
		previous = m.sel.Node.Name
	}
	m.mu.Unlock()

	t := core.Transport(transport)
	tried := map[string]bool{}
	if previous != "" {
		tried[previous] = true
	}
	var selected core.Selection
	var rtt int
	reachable := false
	for {
		candidate, err := core.SelectNodeExcluding(t, tried)
		if err != nil {
			break
		}
		selected = candidate
		tried[candidate.Node.Name] = true
		for attempt := 1; attempt <= 3; attempt++ {
			m.send(ScanEvent{Attempt: attempt, Max: 3, Host: selected.Node.Host})
			rtt, err = ping.Ping(selected.Node.Host, 2000)
			if err == nil {
				reachable = true
				break
			}
		}
		if reachable {
			break
		}
		m.log("warn", fmt.Sprintf("节点 %s 无响应，自动切换下一节点", selected.Node.Name))
	}
	if !reachable && previous != "" {
		if node, ok := core.NodeByName(previous); ok {
			candidate, err := core.BuildSelection(t, node)
			if err == nil {
				selected = candidate
				for attempt := 1; attempt <= 3; attempt++ {
					m.send(ScanEvent{Attempt: attempt, Max: 3, Host: selected.Node.Host})
					rtt, err = ping.Ping(selected.Node.Host, 2000)
					if err == nil {
						reachable = true
						break
					}
				}
			}
		}
	}
	if selected.Node.Name == "" {
		return SelectResult{}, errors.New("没有可用节点")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shutdown || m.state != StateIdle {
		return SelectResult{}, errors.New("已连接状态下无法切换传输")
	}
	m.sel = &selected
	return SelectResult{
		NodeLabel: selected.Label,
		NodeName:  selected.Node.Name,
		Host:      selected.Node.Host,
		Port:      selected.Port,
		Latency:   rtt,
		Reachable: reachable,
	}, nil
}

func (m *Manager) PingSelected() PingResult {
	m.mu.Lock()
	if m.sel == nil {
		m.mu.Unlock()
		return PingResult{Err: errors.New("未选择传输方式")}
	}
	host := m.sel.Node.Host
	m.mu.Unlock()
	var err error
	for range 2 {
		var rtt int
		rtt, err = ping.Ping(host, 2000)
		if err == nil {
			return PingResult{Latency: rtt, Reachable: true}
		}
	}
	return PingResult{Reachable: false, Err: err}
}

func (m *Manager) Connect() error {
	m.mu.Lock()
	if m.shutdown {
		m.mu.Unlock()
		return errors.New("程序正在退出")
	}
	if m.state != StateIdle {
		m.mu.Unlock()
		return errors.New("连接操作正在进行或已经连接")
	}
	if m.sel == nil {
		m.mu.Unlock()
		return errors.New("请先选择传输方式")
	}
	m.state = StateConnecting
	selected := *m.sel
	opts := m.opts
	m.mu.Unlock()

	method := "http"
	if opts.UseSocks5 {
		method = "socks5"
	}
	listenAddr := "127.0.0.1"
	if opts.AllowLAN {
		listenAddr = "0.0.0.0"
	}
	base := core.LocalProxyPort
	if opts.LocalPort >= 1024 {
		base = opts.LocalPort
	}
	end := min(base+core.LocalProxyPortMaxTry, 65536)
	var instance any
	var startErr error
	localPort := 0
	for port := base; port < end; port++ {
		instance, startErr = startProxy(&selected, method, listenAddr, port)
		if startErr == nil {
			localPort = port
			break
		}
		instance = nil
		if !isPortBusy(startErr) {
			m.setIdle()
			return fmt.Errorf("启动代理失败: %w", startErr)
		}
	}
	if instance == nil {
		m.setIdle()
		return fmt.Errorf("本地端口 %d-%d 均被占用: %w", base, end-1, startErr)
	}
	if localPort != base {
		m.log("warn", fmt.Sprintf("端口 %d 被占用，已顺延至 %d", base, localPort))
	}
	m.log("info", fmt.Sprintf("本地代理已启动: %s://%s:%d -> %s:%d", method, listenAddr, localPort, selected.Node.Host, selected.Port))

	if opts.SysProxy {
		if err := m.systemProxy.Enable(fmt.Sprintf("127.0.0.1:%d", localPort)); err != nil {
			_ = m.systemProxy.Restore()
			closeProxy(instance)
			m.setIdle()
			return fmt.Errorf("设置系统代理失败: %w", err)
		}
		m.log("info", "系统代理已开启")
	}
	m.mu.Lock()
	if m.shutdown || m.state != StateConnecting {
		m.mu.Unlock()
		if opts.SysProxy {
			_ = m.systemProxy.Restore()
		}
		closeProxy(instance)
		return errors.New("连接已取消")
	}
	m.proxy = instance
	m.localPort = localPort
	m.sysProxyOn = opts.SysProxy
	m.state = StateConnected
	m.mu.Unlock()
	return nil
}

func (m *Manager) setIdle() {
	m.mu.Lock()
	m.state = StateIdle
	m.mu.Unlock()
}

func startProxy(selected *core.Selection, method, listenAddr string, listenPort int) (any, error) {
	if selected.Transport == core.TransportWebTransport {
		listenURL := fmt.Sprintf("%s://%s:%d", method, listenAddr, listenPort)
		forwardURL := fmt.Sprintf("relay+wt://dwg:%s@%s:%d?secure=true", selected.Node.Password, selected.Node.Host, core.GostServerPort)
		return core.StartGost(listenURL, forwardURL)
	}
	return core.StartHy2(selected.Node.Host, selected.Port, selected.Node.Password, selected.Transport.ObfsPassword(), method, listenAddr, listenPort)
}

func isPortBusy(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "address already in use") ||
		strings.Contains(text, "only one usage of each socket address") ||
		strings.Contains(text, "bind: address in use")
}

func (m *Manager) Disconnect() error {
	m.mu.Lock()
	if m.state != StateConnected {
		m.mu.Unlock()
		return errors.New("当前未连接")
	}
	m.state = StateDisconnecting
	proxyInstance := m.proxy
	relayInstance := m.relay
	hadRelay := m.relayOn
	hadSystemProxy := m.sysProxyOn
	m.proxy = nil
	m.relay = nil
	m.relayOn = false
	m.relayPort = 0
	m.sysProxyOn = false
	m.mu.Unlock()

	if relayInstance != nil {
		relayInstance.Close()
		if hadRelay {
			m.log("info", "SS 中继已关闭")
		}
	}
	closeProxy(proxyInstance)
	if hadSystemProxy {
		if err := m.systemProxy.Restore(); err != nil {
			m.log("warn", fmt.Sprintf("恢复系统代理失败: %v", err))
		} else {
			m.log("info", "系统代理已恢复")
		}
	}
	m.setIdle()
	return nil
}

func closeProxy(instance any) {
	switch proxyInstance := instance.(type) {
	case *core.GostInstance:
		proxyInstance.Close()
	case *core.Hy2Instance:
		proxyInstance.Close()
	}
}

func (m *Manager) StartRelay(port int, password string) error {
	if port <= 0 || port > 65535 {
		return errors.New("中继端口无效")
	}
	if password == "" {
		return errors.New("中继密码不能为空")
	}
	m.mu.Lock()
	if m.state != StateConnected {
		m.mu.Unlock()
		return errors.New("建立连接后才可以开启 SS 中继")
	}
	if m.relayOn {
		m.mu.Unlock()
		return errors.New("SS 中继已经开启")
	}
	method := "http"
	if m.opts.UseSocks5 {
		method = "socks5"
	}
	userinfo := url.UserPassword("aes-128-gcm", password)
	listenURL := fmt.Sprintf("ss://%s@0.0.0.0:%d", userinfo.String(), port)
	forwardURL := fmt.Sprintf("%s://127.0.0.1:%d", method, m.localPort)
	instance, err := core.StartGost(listenURL, forwardURL)
	if err != nil {
		m.mu.Unlock()
		return fmt.Errorf("启动中继失败: %w", err)
	}
	m.relay = instance
	m.relayOn = true
	m.relayPort = port
	m.mu.Unlock()
	m.log("info", fmt.Sprintf("SS 中继已开启: 0.0.0.0:%d (aes-128-gcm)", port))
	return nil
}

func (m *Manager) StopRelay() error {
	m.mu.Lock()
	if !m.relayOn {
		m.mu.Unlock()
		return errors.New("SS 中继未开启")
	}
	instance := m.relay
	m.relay = nil
	m.relayOn = false
	m.relayPort = 0
	m.mu.Unlock()
	instance.Close()
	m.log("info", "SS 中继已关闭")
	return nil
}

func (m *Manager) Status() StatusInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	info := StatusInfo{
		State:       stateName(m.state),
		LocalPort:   m.localPort,
		SysProxy:    m.sysProxyOn,
		RelayActive: m.relayOn,
		RelayPort:   m.relayPort,
		Method:      "http",
		ListenAddr:  "127.0.0.1",
	}
	if m.sel != nil {
		info.Transport = string(m.sel.Transport)
		info.NodeLabel = m.sel.Label
		info.NodeName = m.sel.Node.Name
		info.Host = m.sel.Node.Host
		info.Port = m.sel.Port
	}
	if m.opts.UseSocks5 {
		info.Method = "socks5"
	}
	if m.opts.AllowLAN {
		info.ListenAddr = "0.0.0.0"
	}
	return info
}

func stateName(state State) string {
	switch state {
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateDisconnecting:
		return "disconnecting"
	default:
		return "idle"
	}
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	m.shutdown = true
	proxyInstance := m.proxy
	relayInstance := m.relay
	hadSystemProxy := m.sysProxyOn
	m.proxy = nil
	m.relay = nil
	m.relayOn = false
	m.relayPort = 0
	m.sysProxyOn = false
	m.state = StateIdle
	m.mu.Unlock()
	if relayInstance != nil {
		relayInstance.Close()
	}
	closeProxy(proxyInstance)
	if hadSystemProxy {
		_ = m.systemProxy.Restore()
	}
}
