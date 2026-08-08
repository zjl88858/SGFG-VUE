package proxy

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"sgfg-ng/internal/core"
	"sgfg-ng/internal/ping"
	"sgfg-ng/internal/sysproxy"
)

type State int

const (
	StateIdle State = iota
	StateConnected
)

type StatusInfo struct {
	State       string `json:"state"`
	Transport   string `json:"transport"`
	NodeLabel   string `json:"nodeLabel"`
	NodeName    string `json:"nodeName"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	LocalPort   int    `json:"localPort"`
	Method      string `json:"method"`
	ListenAddr  string `json:"listenAddr"`
	SysProxy    bool   `json:"sysProxy"`
	RelayActive bool   `json:"relayActive"`
	RelayPort   int    `json:"relayPort"`
}

type SelectResult struct {
	NodeLabel string `json:"nodeLabel"`
	NodeName  string `json:"nodeName"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Latency   int    `json:"latency"`
	Reachable bool   `json:"reachable"`
	Err       string `json:"err"`
}

type PingResult struct {
	Latency   int    `json:"latency"`
	Reachable bool   `json:"reachable"`
	Err       string `json:"err"`
}

type ConnectOptions struct {
	UseSocks5 bool `json:"useSocks5"`
	AllowLAN  bool `json:"allowLan"`
	SysProxy  bool `json:"sysProxy"`
	LocalPort int  `json:"localPort"`
}

type Manager struct {
	mu        sync.Mutex
	state     State
	sel       *core.Selection
	opts      ConnectOptions
	proxy     any // *core.GostInstance or *core.Hy2Instance
	localPort int
	relay     *core.GostInstance
	relayOn   bool
	relayPort int
	sysProxy  bool

	emit func(name string, data ...any)
}

func NewManager() *Manager {
	return &Manager{state: StateIdle, opts: ConnectOptions{SysProxy: true}}
}

func (m *Manager) SetOptions(opts ConnectOptions) error {
	if opts.LocalPort != 0 && (opts.LocalPort < 1024 || opts.LocalPort > 65535) {
		return fmt.Errorf("端口需在 1024-65535 之间，或为 0（默认 %d）", core.LocalProxyPort)
	}
	m.mu.Lock()
	m.opts = opts
	m.mu.Unlock()
	if m.emit != nil {
		m.emit("sgfg:options", opts)
	}
	return nil
}

func (m *Manager) Options() ConnectOptions {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.opts
}

func (m *Manager) SetEmitter(f func(name string, data ...any)) {
	m.emit = f
}

func (m *Manager) log(level, msg string) {
	if m.emit != nil {
		m.emit("sgfg:log", map[string]string{"level": level, "msg": msg})
	}
}

func (m *Manager) emitState() {
	if m.emit != nil {
		m.emit("sgfg:state", m.statusLocked())
	}
}

func (m *Manager) SelectTransport(transport string) SelectResult {
	m.mu.Lock()
	if m.state != StateIdle {
		m.mu.Unlock()
		return SelectResult{Err: "已连接状态下无法切换传输"}
	}
	prev := ""
	if m.sel != nil {
		prev = m.sel.Node.Name
	}
	m.mu.Unlock()

	t := core.Transport(transport)
	tried := map[string]bool{prev: true}
	var sel core.Selection
	rtt := 0
	reachable := false

	for {
		cand, err := core.SelectNodeExcluding(t, tried)
		if err != nil {
			break
		}
		sel = cand
		tried[cand.Node.Name] = true

		for i := 1; i <= 3; i++ {
			if m.emit != nil {
				m.emit("sgfg:scan", map[string]any{"attempt": i, "max": 3, "host": sel.Node.Host})
			}
			var perr error
			rtt, perr = ping.Ping(sel.Node.Host, 2000)
			if perr == nil {
				reachable = true
				break
			}
		}
		if reachable {
			break
		}
		m.log("warn", fmt.Sprintf("节点 %s 无响应，自动切换下一节点", sel.Node.Name))
	}

	if !reachable && prev != "" {
		if pn, ok := core.NodeByName(prev); ok {
			if psel, err := core.BuildSelection(t, pn); err == nil {
				sel = psel
				for i := 1; i <= 3; i++ {
					if m.emit != nil {
						m.emit("sgfg:scan", map[string]any{"attempt": i, "max": 3, "host": sel.Node.Host})
					}
					var perr error
					rtt, perr = ping.Ping(sel.Node.Host, 2000)
					if perr == nil {
						reachable = true
						break
					}
				}
			}
		}
	}

	if sel.Node.Name == "" {
		return SelectResult{Err: "没有可用节点"}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != StateIdle {
		return SelectResult{Err: "已连接状态下无法切换传输"}
	}
	m.sel = &sel

	return SelectResult{
		NodeLabel: sel.Label,
		NodeName:  sel.Node.Name,
		Host:      sel.Node.Host,
		Port:      sel.Port,
		Latency:   rtt,
		Reachable: reachable,
	}
}

func pingWithRetry(host string, timeoutMs, attempts int) (int, error) {
	var err error
	var rtt int
	for i := 0; i < attempts; i++ {
		rtt, err = ping.Ping(host, timeoutMs)
		if err == nil {
			return rtt, nil
		}
	}
	return 0, err
}

func (m *Manager) PingSelected() PingResult {
	m.mu.Lock()
	sel := m.sel
	m.mu.Unlock()
	if sel == nil {
		return PingResult{Err: "未选择传输方式"}
	}
	rtt, err := pingWithRetry(sel.Node.Host, 2000, 2)
	if err != nil {
		return PingResult{Reachable: false}
	}
	return PingResult{Latency: rtt, Reachable: true}
}

func (m *Manager) Selected() *core.Selection {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sel
}

func (m *Manager) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != StateIdle {
		return errors.New("already connected")
	}
	if m.sel == nil {
		return errors.New("请先选择传输方式")
	}
	opts := m.opts

	method := "http"
	if opts.UseSocks5 {
		method = "socks5"
	}
	listenAddr := "127.0.0.1"
	if opts.AllowLAN {
		listenAddr = "0.0.0.0"
	}

	sel := m.sel
	base := core.LocalProxyPort
	if opts.LocalPort >= 1024 {
		base = opts.LocalPort
	}
	end := base + core.LocalProxyPortMaxTry
	if end > 65536 {
		end = 65536
	}
	var proxyInst any
	localPort := 0
	var startErr error
	for p := base; p < end; p++ {
		proxyInst, startErr = startProxy(sel, method, listenAddr, p)
		if startErr == nil {
			localPort = p
			break
		}
		proxyInst = nil
		if !isPortBusy(startErr) {
			return fmt.Errorf("启动代理失败: %w", startErr)
		}
	}
	if proxyInst == nil {
		return fmt.Errorf("本地端口 %d-%d 均被占用: %w", base, end-1, startErr)
	}
	m.proxy = proxyInst
	m.localPort = localPort
	if localPort != base {
		m.log("warn", fmt.Sprintf("端口 %d 被占用，已顺延至 %d", base, localPort))
	}
	m.log("info", fmt.Sprintf("本地代理已启动: %s://%s:%d -> %s:%d", method, listenAddr, localPort, sel.Node.Host, sel.Port))

	if opts.SysProxy {
		if err := sysproxy.On(fmt.Sprintf("127.0.0.1:%d", localPort)); err != nil {
			m.closeProxyLocked()
			return fmt.Errorf("设置系统代理失败: %w", err)
		}
		m.sysProxy = true
		m.log("info", "系统代理已开启")
	}

	m.state = StateConnected
	m.emitState()
	return nil
}

func startProxy(sel *core.Selection, method, listenAddr string, listenPort int) (any, error) {
	switch sel.Transport {
	case core.TransportWebTransport:
		listenURL := fmt.Sprintf("%s://%s:%d", method, listenAddr, listenPort)
		forwardURL := fmt.Sprintf("relay+wt://dwg:%s@%s:%d?secure=true", sel.Node.Password, sel.Node.Host, core.GostServerPort)
		return core.StartGost(listenURL, forwardURL)
	default:
		return core.StartHy2(sel.Node.Host, sel.Port, sel.Node.Password, sel.Transport.ObfsPassword(), method, listenAddr, listenPort)
	}
}

func isPortBusy(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "Only one usage of each socket address") ||
		strings.Contains(s, "address already in use")
}

func (m *Manager) Disconnect() error {
	m.mu.Lock()
	if m.state != StateConnected {
		m.mu.Unlock()
		return errors.New("not connected")
	}

	m.state = StateIdle
	proxyInst := m.proxy
	relayInst := m.relay
	hadRelay := m.relayOn
	hadSysProxy := m.sysProxy
	m.proxy = nil
	m.relay = nil
	m.relayOn = false
	m.relayPort = 0
	m.sysProxy = false
	m.mu.Unlock()

	if hadRelay && relayInst != nil {
		relayInst.Close()
		m.log("info", "SS 中继已关闭")
	}
	switch p := proxyInst.(type) {
	case *core.GostInstance:
		p.Close()
		m.log("info", "GOST 已停止")
	case *core.Hy2Instance:
		p.Close()
		m.log("info", "Hysteria2 已停止")
	}

	if hadSysProxy {
		if err := sysproxy.Off(); err != nil {
			m.log("warn", fmt.Sprintf("关闭系统代理失败: %v", err))
		} else {
			m.log("info", "系统代理已关闭")
		}
	}

	m.emitState()
	return nil
}

func (m *Manager) closeProxyLocked() {
	switch p := m.proxy.(type) {
	case *core.GostInstance:
		p.Close()
		m.log("info", "GOST 已停止")
	case *core.Hy2Instance:
		p.Close()
		m.log("info", "Hysteria2 已停止")
	}
	m.proxy = nil
}

func (m *Manager) StartRelay(port int, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != StateConnected {
		return errors.New("建立连接后才可以开启SS中继")
	}
	if m.relayOn {
		return errors.New("relay already running")
	}
	if port <= 0 || port > 65535 {
		return errors.New("端口无效")
	}
	if password == "" {
		return errors.New("密码不能为空")
	}

	method := "http"
	if m.opts.UseSocks5 {
		method = "socks5"
	}
	userinfo := url.UserPassword("aes-128-gcm", password)
	listenURL := fmt.Sprintf("ss://%s@0.0.0.0:%d", userinfo.String(), port)
	forwardURL := fmt.Sprintf("%s://127.0.0.1:%d", method, m.localPort)
	inst, err := core.StartGost(listenURL, forwardURL)
	if err != nil {
		return fmt.Errorf("启动中继失败: %w", err)
	}
	m.relay = inst
	m.relayOn = true
	m.relayPort = port
	m.log("info", fmt.Sprintf("SS 中继已开启: 0.0.0.0:%d (aes-128-gcm)", port))
	m.emitState()
	return nil
}

func (m *Manager) StopRelay() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.relayOn {
		return errors.New("relay not running")
	}
	m.stopRelayLocked()
	m.emitState()
	return nil
}

func (m *Manager) stopRelayLocked() {
	if m.relay != nil {
		m.relay.Close()
		m.relay = nil
	}
	m.relayOn = false
	m.relayPort = 0
	m.log("info", "SS 中继已关闭")
}

func (m *Manager) Status() StatusInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked()
}

func (m *Manager) statusLocked() StatusInfo {
	info := StatusInfo{
		State:       "idle",
		SysProxy:    m.sysProxy,
		RelayActive: m.relayOn,
		RelayPort:   m.relayPort,
	}
	if m.state == StateConnected {
		info.State = "connected"
	}
	if m.sel != nil {
		info.Transport = string(m.sel.Transport)
		info.NodeLabel = m.sel.Label
		info.NodeName = m.sel.Node.Name
		info.Host = m.sel.Node.Host
		info.Port = m.sel.Port
	}
	info.LocalPort = m.localPort
	if m.opts.UseSocks5 {
		info.Method = "socks5"
	} else {
		info.Method = "http"
	}
	if m.opts.AllowLAN {
		info.ListenAddr = "0.0.0.0"
	} else {
		info.ListenAddr = "127.0.0.1"
	}
	return info
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	proxyInst := m.proxy
	relayInst := m.relay
	hadSysProxy := m.sysProxy
	m.proxy = nil
	m.relay = nil
	m.relayOn = false
	m.relayPort = 0
	m.sysProxy = false
	m.state = StateIdle
	m.mu.Unlock()

	if relayInst != nil {
		relayInst.Close()
	}
	switch p := proxyInst.(type) {
	case *core.GostInstance:
		p.Close()
	case *core.Hy2Instance:
		p.Close()
	}
	if hadSysProxy {
		sysproxy.Off()
	}
}
