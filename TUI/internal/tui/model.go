package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sgfg-tui/internal/core"
	"sgfg-tui/internal/proxy"
)

const Version = "v11.2-tui"

type page int

const (
	homePage page = iota
	optionsPage
	relayPage
)

type operationDoneMsg struct {
	kind      string
	err       error
	selection proxy.SelectResult
	status    proxy.StatusInfo
	options   proxy.ConnectOptions
}

type pingDoneMsg struct {
	result proxy.PingResult
}

type eventMsg struct{ value any }
type tickMsg time.Time
type QuitMsg struct{}

type logLine struct {
	level   string
	message string
}

type Model struct {
	manager *proxy.Manager
	events  <-chan any

	page          page
	width         int
	height        int
	status        proxy.StatusInfo
	options       proxy.ConnectOptions
	sysAvailable  bool
	sysReason     string
	active        string
	latency       int
	reachable     bool
	hasLatency    bool
	busy          bool
	busyLabel     string
	pinging       bool
	scanHost      string
	scanAttempt   int
	toast         string
	toastError    bool
	modalOpen     bool
	modalTitle    string
	modalMessage  string
	modalError    bool
	logs          []logLine
	logOffset     int
	optionsCursor int
	relayCursor   int

	spinner       spinner.Model
	localPort     textinput.Model
	relayPort     textinput.Model
	relayPassword textinput.Model
}

func NewModel(manager *proxy.Manager, events <-chan any) Model {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(green).Background(surface)

	localPort := textinput.New()
	localPort.Prompt = "› "
	localPort.PromptStyle = lipgloss.NewStyle().Foreground(cyan)
	localPort.TextStyle = lipgloss.NewStyle().Foreground(text)
	localPort.PlaceholderStyle = lipgloss.NewStyle().Foreground(muted)
	localPort.Cursor.Style = lipgloss.NewStyle().Foreground(lime)
	localPort.Placeholder = strconv.Itoa(core.LocalProxyPort)
	localPort.CharLimit = 5
	localPort.Width = 12

	relayPort := textinput.New()
	relayPort.Prompt = "› "
	relayPort.PromptStyle = lipgloss.NewStyle().Foreground(cyan)
	relayPort.TextStyle = lipgloss.NewStyle().Foreground(text)
	relayPort.Cursor.Style = lipgloss.NewStyle().Foreground(lime)
	relayPort.SetValue("8388")
	relayPort.CharLimit = 5
	relayPort.Width = 10

	relayPassword := textinput.New()
	relayPassword.Prompt = "› "
	relayPassword.PromptStyle = lipgloss.NewStyle().Foreground(cyan)
	relayPassword.TextStyle = lipgloss.NewStyle().Foreground(text)
	relayPassword.PlaceholderStyle = lipgloss.NewStyle().Foreground(muted)
	relayPassword.Cursor.Style = lipgloss.NewStyle().Foreground(lime)
	relayPassword.Placeholder = "输入或生成密码"
	relayPassword.CharLimit = 64
	relayPassword.Width = 28

	options := manager.Options()
	if options.LocalPort > 0 {
		localPort.SetValue(strconv.Itoa(options.LocalPort))
	}
	available, reason := manager.SystemProxyCapability()
	return Model{
		manager:       manager,
		events:        events,
		status:        manager.Status(),
		options:       options,
		sysAvailable:  available,
		sysReason:     reason,
		spinner:       spin,
		localPort:     localPort,
		relayPort:     relayPort,
		relayPassword: relayPassword,
		reachable:     true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(waitEvent(m.events), tick(), m.spinner.Tick)
}

func waitEvent(events <-chan any) tea.Cmd {
	return func() tea.Msg {
		value, ok := <-events
		if !ok {
			return nil
		}
		return eventMsg{value: value}
	}
}

func tick() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	var commands []tea.Cmd
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case spinner.TickMsg:
		var command tea.Cmd
		m.spinner, command = m.spinner.Update(msg)
		commands = append(commands, command)
	case eventMsg:
		switch event := msg.value.(type) {
		case proxy.LogEvent:
			if m.logOffset > 0 {
				m.logOffset++
			}
			m.logs = append([]logLine{{level: event.Level, message: normalizeLogMessage(event.Message)}}, m.logs...)
			if len(m.logs) > 60 {
				m.logs = m.logs[:60]
			}
		case proxy.ScanEvent:
			m.scanHost = event.Host
			m.scanAttempt = event.Attempt
		}
		commands = append(commands, waitEvent(m.events))
	case operationDoneMsg:
		m.busy = false
		m.busyLabel = ""
		m.status = msg.status
		m.options = msg.options
		if msg.err != nil {
			m.showToast(msg.err.Error(), true)
			if msg.kind == "select" {
				m.active = m.status.Transport
			}
			break
		}
		switch msg.kind {
		case "select":
			m.active = m.status.Transport
			m.latency = msg.selection.Latency
			m.reachable = msg.selection.Reachable
			m.hasLatency = true
			if msg.selection.Reachable {
				m.showToast(fmt.Sprintf("节点可用，延迟 %dms", msg.selection.Latency), false)
			} else {
				m.showToast("ICMP 探测失败；仍可尝试连接，或重新选择节点", true)
			}
		case "connect":
			m.showToast("已连接", false)
		case "disconnect":
			m.showToast("已断开", false)
		case "options":
			m.syncLocalPort()
			m.showToast("选项已更新", false)
		case "relay-start":
			m.showToast("SS 中继已开启，供局域网设备使用", false)
		case "relay-stop":
			m.showToast("SS 中继已关闭", false)
		}
	case pingDoneMsg:
		m.pinging = false
		if msg.result.Err == nil || !msg.result.Reachable {
			m.latency = msg.result.Latency
			m.reachable = msg.result.Reachable
			m.hasLatency = true
		}
	case tickMsg:
		commands = append(commands, tick())
		if m.status.Transport != "" && !m.busy && !m.pinging {
			m.pinging = true
			commands = append(commands, m.pingCmd())
		}
	case QuitMsg:
		return m, tea.Quit
	case tea.MouseMsg:
		if command := m.handleMouse(msg); command != nil {
			commands = append(commands, command)
		}
	case tea.KeyMsg:
		if command := m.handleKey(msg); command != nil {
			commands = append(commands, command)
		}
	}
	return m, tea.Batch(commands...)
}

func (m *Model) handleMouse(message tea.MouseMsg) tea.Cmd {
	event := tea.MouseEvent(message)
	if m.modalOpen {
		if event.Action == tea.MouseActionPress && event.Button == tea.MouseButtonLeft &&
			modalButtonRect(m.width, m.height).contains(event.X, event.Y) {
			m.closeModal()
		}
		return nil
	}
	layout := calculateLayout(m.width, m.height)
	if event.Button == tea.MouseButtonWheelUp || event.Button == tea.MouseButtonWheelDown {
		if m.page == homePage && layout.logs.contains(event.X, event.Y) {
			if event.Button == tea.MouseButtonWheelUp {
				m.logOffset = min(max(0, len(m.logs)-1), m.logOffset+3)
			} else {
				m.logOffset = max(0, m.logOffset-3)
			}
		}
		return nil
	}
	if event.Action != tea.MouseActionPress || event.Button != tea.MouseButtonLeft {
		return nil
	}
	for i, target := range layout.tabs {
		if target.contains(event.X, event.Y) {
			m.blurInputs()
			m.page = page(i)
			return nil
		}
	}
	if m.busy {
		return nil
	}
	switch m.page {
	case homePage:
		for i, target := range layout.transports {
			if target.contains(event.X, event.Y) {
				return m.handleHomeKey(strconv.Itoa(i + 1))
			}
		}
		if layout.connect.contains(event.X, event.Y) {
			return m.handleHomeKey("c")
		}
	case optionsPage:
		for i, target := range layout.options {
			if target.contains(event.X, event.Y) {
				m.blurInputs()
				m.optionsCursor = i
				return m.handleOptionsKey("enter")
			}
		}
	case relayPage:
		for i, target := range layout.relay {
			if target.contains(event.X, event.Y) {
				m.blurInputs()
				m.relayCursor = i
				return m.handleRelayKey("enter")
			}
		}
	}
	return nil
}

func (m *Model) handleKey(key tea.KeyMsg) tea.Cmd {
	if m.modalOpen {
		switch key.String() {
		case "ctrl+c":
			return tea.Quit
		case "enter", " ", "esc", "q":
			m.closeModal()
		}
		return nil
	}
	if m.inputFocused() {
		if key.String() == "esc" {
			m.blurInputs()
			return nil
		}
		if key.String() == "enter" {
			if m.page == optionsPage {
				m.blurInputs()
				return m.savePort()
			}
			m.blurInputs()
			return nil
		}
		return m.updateFocusedInput(key)
	}

	switch key.String() {
	case "ctrl+c", "q":
		return tea.Quit
	case "tab", "right":
		m.page = (m.page + 1) % 3
		return nil
	case "shift+tab", "left":
		m.page = (m.page + 2) % 3
		return nil
	case "h":
		m.page = homePage
		return nil
	case "o":
		m.page = optionsPage
		return nil
	case "r":
		m.page = relayPage
		return nil
	}
	if m.busy {
		return nil
	}
	switch m.page {
	case homePage:
		return m.handleHomeKey(key.String())
	case optionsPage:
		return m.handleOptionsKey(key.String())
	case relayPage:
		return m.handleRelayKey(key.String())
	}
	return nil
}

func (m *Model) handleHomeKey(key string) tea.Cmd {
	transports := map[string]string{"1": "webtransport", "2": "hysteria2", "3": "turtleor"}
	if transport := transports[key]; transport != "" {
		if m.status.State != "idle" {
			m.showToast("请先断开连接再切换传输方式", true)
			return nil
		}
		m.active = transport
		m.busy = true
		m.busyLabel = "正在探测并分配节点"
		m.scanAttempt = 0
		m.scanHost = ""
		m.hasLatency = false
		return m.selectCmd(transport)
	}
	if key == "c" || key == "enter" {
		m.busy = true
		if m.status.State == "connected" {
			m.busyLabel = "正在断开连接"
			return m.operationCmd("disconnect", m.manager.Disconnect)
		}
		if m.status.Transport == "" {
			m.busy = false
			m.showToast("请先选择传输方式", true)
			return nil
		}
		m.busyLabel = "正在建立本地代理"
		return m.operationCmd("connect", m.manager.Connect)
	}
	return nil
}

func (m *Model) handleOptionsKey(key string) tea.Cmd {
	switch key {
	case "up", "k":
		m.optionsCursor = (m.optionsCursor + 3) % 4
	case "down", "j":
		m.optionsCursor = (m.optionsCursor + 1) % 4
	case " ", "enter":
		if m.status.State != "idle" {
			m.showToast("连接期间无法修改代理选项", true)
			return nil
		}
		switch m.optionsCursor {
		case 0:
			if !m.sysAvailable {
				m.showToast(m.sysReason, true)
				return nil
			}
			options := m.options
			options.SysProxy = !options.SysProxy
			if options.SysProxy {
				options.UseSocks5 = false
			}
			return m.optionsCmd(options)
		case 1:
			options := m.options
			options.AllowLAN = !options.AllowLAN
			return m.optionsCmd(options)
		case 2:
			options := m.options
			options.UseSocks5 = !options.UseSocks5
			if options.UseSocks5 {
				options.SysProxy = false
			}
			return m.optionsCmd(options)
		case 3:
			m.localPort.Focus()
			return textinput.Blink
		}
	}
	return nil
}

func (m *Model) handleRelayKey(key string) tea.Cmd {
	switch key {
	case "up", "k":
		m.relayCursor = (m.relayCursor + 3) % 4
	case "down", "j":
		m.relayCursor = (m.relayCursor + 1) % 4
	case "g":
		m.relayPassword.SetValue(core.GeneratePassword())
		m.showToast("已生成随机中继密码", false)
	case " ", "enter":
		switch m.relayCursor {
		case 0:
			if m.status.RelayActive {
				m.busy = true
				m.busyLabel = "正在关闭 SS 中继"
				return m.operationCmd("relay-stop", m.manager.StopRelay)
			}
			if m.status.State != "connected" {
				m.showToast("建立连接后才可以开启 SS 中继", true)
				return nil
			}
			port, err := strconv.Atoi(strings.TrimSpace(m.relayPort.Value()))
			if err != nil || port < 1 || port > 65535 {
				m.showToast("中继端口无效", true)
				return nil
			}
			if m.relayPassword.Value() == "" {
				m.showToast("请先输入或生成中继密码", true)
				return nil
			}
			m.busy = true
			m.busyLabel = "正在开启 SS 中继"
			return m.operationCmd("relay-start", func() error {
				return m.manager.StartRelay(port, m.relayPassword.Value())
			})
		case 1:
			if !m.status.RelayActive {
				m.relayPort.Focus()
				return textinput.Blink
			}
		case 2:
			if !m.status.RelayActive {
				m.relayPassword.Focus()
				return textinput.Blink
			}
		case 3:
			if !m.status.RelayActive {
				m.relayPassword.SetValue(core.GeneratePassword())
				m.showToast("已生成随机中继密码", false)
			}
		}
	}
	return nil
}

func (m *Model) savePort() tea.Cmd {
	value := strings.TrimSpace(m.localPort.Value())
	port := 0
	var err error
	if value != "" {
		port, err = strconv.Atoi(value)
	}
	if err != nil || (port != 0 && (port < 1024 || port > 65535)) {
		m.showToast("端口需为 1024-65535，或留空使用默认 7788", true)
		m.syncLocalPort()
		return nil
	}
	options := m.options
	options.LocalPort = port
	return m.optionsCmd(options)
}

func (m *Model) optionsCmd(options proxy.ConnectOptions) tea.Cmd {
	m.busy = true
	m.busyLabel = "正在保存选项"
	return func() tea.Msg {
		err := m.manager.SetOptions(options)
		return operationDoneMsg{
			kind:    "options",
			err:     err,
			status:  m.manager.Status(),
			options: m.manager.Options(),
		}
	}
}

func (m Model) operationCmd(kind string, operation func() error) tea.Cmd {
	return func() tea.Msg {
		err := operation()
		return operationDoneMsg{
			kind:    kind,
			err:     err,
			status:  m.manager.Status(),
			options: m.manager.Options(),
		}
	}
}

func (m Model) selectCmd(transport string) tea.Cmd {
	return func() tea.Msg {
		selection, err := m.manager.SelectTransport(transport)
		return operationDoneMsg{
			kind:      "select",
			err:       err,
			selection: selection,
			status:    m.manager.Status(),
			options:   m.manager.Options(),
		}
	}
}

func (m Model) pingCmd() tea.Cmd {
	return func() tea.Msg { return pingDoneMsg{result: m.manager.PingSelected()} }
}

func (m *Model) inputFocused() bool {
	return m.localPort.Focused() || m.relayPort.Focused() || m.relayPassword.Focused()
}

func (m *Model) blurInputs() {
	m.localPort.Blur()
	m.relayPort.Blur()
	m.relayPassword.Blur()
}

func (m *Model) updateFocusedInput(key tea.KeyMsg) tea.Cmd {
	var command tea.Cmd
	if m.localPort.Focused() {
		m.localPort, command = m.localPort.Update(key)
	} else if m.relayPort.Focused() {
		m.relayPort, command = m.relayPort.Update(key)
	} else if m.relayPassword.Focused() {
		m.relayPassword, command = m.relayPassword.Update(key)
	}
	return command
}

func (m *Model) syncLocalPort() {
	if m.options.LocalPort == 0 {
		m.localPort.SetValue("")
	} else {
		m.localPort.SetValue(strconv.Itoa(m.options.LocalPort))
	}
}

func (m *Model) showToast(message string, isError bool) {
	if isError {
		m.toast = ""
		m.toastError = false
		m.modalOpen = true
		m.modalTitle = "ACTION REQUIRED"
		m.modalMessage = message
		m.modalError = true
		m.blurInputs()
		return
	}
	m.toast = message
	m.toastError = isError
}

func (m *Model) closeModal() {
	m.modalOpen = false
	m.modalTitle = ""
	m.modalMessage = ""
	m.modalError = false
}
