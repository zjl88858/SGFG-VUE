package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	green     = lipgloss.Color("#56F287")
	cyan      = lipgloss.Color("#35DDF2")
	lime      = lipgloss.Color("#D7FF4F")
	red       = lipgloss.Color("#FF5F6D")
	text      = lipgloss.Color("#D6E4D8")
	muted     = lipgloss.Color("#819184")
	subtle    = lipgloss.Color("#263229")
	base      = lipgloss.Color("#080D09")
	sidebarBG = lipgloss.Color("#0B120D")
	surface   = lipgloss.Color("#101A12")

	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(green)
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(green)
	accentStyle  = lipgloss.NewStyle().Bold(true).Foreground(cyan)
	keyStyle     = lipgloss.NewStyle().Bold(true).Foreground(lime)
	mutedStyle   = lipgloss.NewStyle().Foreground(muted)
	goodStyle    = lipgloss.NewStyle().Bold(true).Foreground(green)
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(red)

	surfaceAccentStyle  = accentStyle.Copy().Background(surface)
	surfaceSectionStyle = sectionStyle.Copy().Background(surface)
	surfaceMutedStyle   = mutedStyle.Copy().Background(surface)
	surfaceGoodStyle    = goodStyle.Copy().Background(surface)
	surfaceErrorStyle   = errorStyle.Copy().Background(surface)
)

func (m Model) View() string {
	if m.width == 0 {
		return "正在启动 SGFG TUI..."
	}
	layout := calculateLayout(m.width, m.height)
	header := m.headerView()
	if m.modalOpen {
		return lipgloss.JoinVertical(lipgloss.Left, header, m.modalBodyView(layout.body.width, layout.body.height), m.footerView())
	}
	main := m.mainView(layout.main.width, layout.main.height)
	body := main
	if layout.wide {
		sidebar := m.sidebarView(layout.sidebar.width, layout.sidebar.height)
		gap := panelLines(2, layout.body.height, nil, base)
		body = lipgloss.JoinHorizontal(lipgloss.Top, main, gap, sidebar)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, m.footerView())
}

func (m Model) headerView() string {
	state := mutedStyle.Render("● idle")
	if m.status.State == "connected" {
		state = goodStyle.Render("● connected")
	} else if m.status.State != "idle" {
		state = keyStyle.Render("◌ " + m.status.State)
	}
	title := "  " + titleStyle.Render("SGFG") + mutedStyle.Render("  /  proxy workspace  "+Version)
	gap := max(1, m.width-lipgloss.Width(title)-lipgloss.Width(state)-2)
	first := title + strings.Repeat(" ", gap) + state + "  "

	labels := []string{" Home ", " Options ", " SS Relay "}
	second := "  "
	for i, label := range labels {
		style := mutedStyle
		if int(m.page) == i {
			style = lipgloss.NewStyle().Bold(true).Foreground(base).Background(green)
		}
		second += style.Render(label)
		if i != len(labels)-1 {
			second += " "
		}
	}
	separator := lipgloss.NewStyle().Foreground(subtle).Render(strings.Repeat("─", max(0, m.width)))
	return lipgloss.JoinVertical(lipgloss.Left,
		barLine(m.width, first, base),
		barLine(m.width, second, base),
		barLine(m.width, separator, base),
	)
}

func (m Model) mainView(width, height int) string {
	switch m.page {
	case optionsPage:
		return m.optionsView(width, height)
	case relayPage:
		return m.relayView(width, height)
	default:
		return m.homeView(width, height)
	}
}

func (m Model) homeView(width, height int) string {
	lines := make([]string, height)
	setLine(lines, 0, "  "+sectionStyle.Render("TRANSPORT"))
	setLine(lines, 1, "  "+mutedStyle.Render("Choose a route, then start the local proxy."))

	type transport struct{ key, name, description string }
	transports := []transport{
		{"webtransport", "[1] WebTransport", "relay+wt direct · HTTP/3"},
		{"hysteria2", "[2] Hysteria2", "hy2 direct · port hopping"},
		{"turtleor", "[3] Turtleor", "hy2 via warpplus(M-H3)"},
	}
	for i, item := range transports {
		y := 3 + i*3
		selected := m.active == item.key || m.status.Transport == item.key
		marker := mutedStyle.Render("│")
		name := lipgloss.NewStyle().Foreground(text).Render(item.name)
		if selected {
			marker = accentStyle.Render("▌")
			name = accentStyle.Render(item.name)
		}
		setLine(lines, y, "  "+marker+"  "+name)
		setLine(lines, y+1, "     "+mutedStyle.Render(item.description))
	}

	setLine(lines, 12, "  "+sectionStyle.Render("ACTIVITY")+"  "+m.logPositionView())
	connectStart := max(0, height-3)
	visibleLogs := max(0, connectStart-13)
	if len(m.logs) == 0 {
		setLine(lines, 13, "  "+mutedStyle.Render("No activity yet. Select a transport to begin."))
	} else {
		start := min(m.logOffset, max(0, len(m.logs)-1))
		for i := 0; i < visibleLogs && start+i < len(m.logs); i++ {
			setLine(lines, 13+i, m.logView(m.logs[start+i]))
		}
	}

	action := "Connect"
	if m.status.State == "connected" {
		action = "Disconnect"
	}
	actionView := surfaceAccentStyle.Render(action)
	detail := "select a transport first"
	if m.status.NodeLabel != "" {
		detail = m.status.NodeLabel
	}
	if m.busy {
		actionView = m.spinner.View() + surfaceAccentStyle.Render("  "+m.busyLabel)
		if m.scanHost != "" {
			detail = fmt.Sprintf("%s · attempt %d/3", m.scanHost, m.scanAttempt)
		}
	}
	if m.toast != "" {
		detail = m.surfaceNotificationView()
	} else {
		detail = surfaceMutedStyle.Render(detail)
	}
	if height >= 3 {
		setLine(lines, connectStart, actionLine(width, ""))
		setLine(lines, connectStart+1, actionLine(width, surfaceFill("  ")+actionView+surfaceMutedStyle.Render("  ·  c / Enter / click")))
		setLine(lines, connectStart+2, actionLine(width, surfaceFill("  ")+detail))
	}
	return panelLines(width, height, lines, base)
}

func (m Model) optionsView(width, height int) string {
	lines := make([]string, height)
	setLine(lines, 0, "  "+sectionStyle.Render("OPTIONS"))
	setLine(lines, 1, "  "+mutedStyle.Render("Connection settings are locked while the proxy is running."))

	values := []string{
		toggle(m.options.SysProxy),
		toggle(m.options.AllowLAN),
		toggle(m.options.UseSocks5),
		m.localPort.View(),
	}
	labels := []string{"Automatic system proxy", "Allow LAN clients", "Use SOCKS5", "Local listen port"}
	hints := []string{m.sysReason, "listen on 0.0.0.0", "mutually exclusive with system proxy", "blank = 7788 · increments when occupied"}
	if !m.sysAvailable {
		values[0] = errorStyle.Render("unavailable")
	}
	for i := range labels {
		y := 3 + i*3
		marker := mutedStyle.Render("│")
		label := lipgloss.NewStyle().Foreground(text).Render(labels[i])
		if m.optionsCursor == i {
			marker = accentStyle.Render("▌")
			label = accentStyle.Render(labels[i])
		}
		setLine(lines, y, "  "+marker+"  "+label+"  "+values[i])
		setLine(lines, y+1, "     "+mutedStyle.Render(hints[i]))
	}
	if m.status.State != "idle" {
		setLine(lines, 16, "  "+errorStyle.Render("LOCKED")+mutedStyle.Render("  disconnect before changing these values"))
	}
	m.setPageNotification(lines, width, height, "↑/↓ select  ·  Space/Enter change")
	return panelLines(width, height, lines, base)
}

func (m Model) relayView(width, height int) string {
	lines := make([]string, height)
	setLine(lines, 0, "  "+sectionStyle.Render("SS RELAY"))
	setLine(lines, 1, "  "+mutedStyle.Render("Share the active connection with devices on your LAN."))

	values := []string{toggle(m.status.RelayActive), m.relayPort.View(), m.relayPassword.View(), keyStyle.Render("generate")}
	labels := []string{"Relay service", "Listen port", "Password", "Random password"}
	hints := []string{"requires an active main connection", "default 8388 · listens on 0.0.0.0", "used by Shadowsocks clients", "cryptographically secure · 8-16 characters"}
	for i := range labels {
		y := 3 + i*3
		marker := mutedStyle.Render("│")
		label := lipgloss.NewStyle().Foreground(text).Render(labels[i])
		if m.relayCursor == i {
			marker = accentStyle.Render("▌")
			label = accentStyle.Render(labels[i])
		}
		setLine(lines, y, "  "+marker+"  "+label+"  "+values[i])
		setLine(lines, y+1, "     "+mutedStyle.Render(hints[i]))
	}
	if m.status.RelayActive {
		setLine(lines, 16, "  "+goodStyle.Render(fmt.Sprintf("ACTIVE  ss://aes-128-gcm · LAN-IP:%d", m.status.RelayPort)))
	}
	m.setPageNotification(lines, width, height, "↑/↓ select  ·  Enter edit or toggle")
	return panelLines(width, height, lines, base)
}

func (m Model) sidebarView(width, height int) string {
	lines := make([]string, height)
	setLine(lines, 0, "  "+titleStyle.Render("SGFG context"))
	setLine(lines, 2, "  "+sectionStyle.Render("CONNECTION"))
	setLine(lines, 3, sidebarValue("State", m.status.State))
	setLine(lines, 4, sidebarValue("Route", valueOr(m.status.Transport, "not selected")))

	setLine(lines, 7, "  "+sectionStyle.Render("NODE"))
	setLine(lines, 8, sidebarValue("Name", valueOr(m.status.NodeName, "—")))
	setLine(lines, 9, sidebarValue("Host", valueOr(m.status.Host, "—")))
	port := "—"
	if m.status.Port != 0 {
		port = fmt.Sprint(m.status.Port)
	}
	setLine(lines, 10, sidebarValue("Port", port))
	latency := "—"
	if m.hasLatency {
		if m.reachable {
			latency = fmt.Sprintf("%d ms", m.latency)
		} else {
			latency = "timeout"
		}
	}
	setLine(lines, 11, sidebarValue("Latency", latency))

	setLine(lines, 14, "  "+sectionStyle.Render("LOCAL PROXY"))
	setLine(lines, 15, sidebarValue("Method", m.status.Method))
	setLine(lines, 16, sidebarValue("Listen", m.localAddress()))
	setLine(lines, 17, sidebarValue("System", onOff(m.options.SysProxy)))
	setLine(lines, 18, sidebarValue("LAN", onOff(m.options.AllowLAN)))
	setLine(lines, 19, sidebarValue("SS relay", onOff(m.status.RelayActive)))

	if height >= 3 {
		setLine(lines, height-3, "  "+accentStyle.Render("~/sgfg-tui"))
		setLine(lines, height-1, "  "+goodStyle.Render("• ")+mutedStyle.Render("Go + Charm · mouse enabled"))
	}
	return panelLines(width, height, lines, sidebarBG)
}

func (m Model) footerView() string {
	left := "  SGFG  ·  local proxy workspace"
	right := "mouse: click / wheel   tab: pages   q: quit  "
	if m.modalOpen {
		right = "enter / esc / click: confirm  "
	}
	if m.inputFocused() {
		right = "enter: confirm   esc: cancel  "
	}
	if m.width < 90 {
		compact := "  click: select  ·  wheel: logs  ·  tab: pages  ·  q: quit"
		if m.modalOpen {
			compact = "  enter / esc / click: confirm"
		}
		if m.inputFocused() {
			compact = "  enter: confirm  ·  esc: cancel"
		}
		return barLine(m.width, mutedStyle.Render(compact), sidebarBG)
	}
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	second := mutedStyle.Render(left) + strings.Repeat(" ", gap) + mutedStyle.Render(right)
	return barLine(m.width, second, sidebarBG)
}

func (m Model) surfaceNotificationView() string {
	if m.toastError {
		return surfaceErrorStyle.Render(m.toast)
	}
	return surfaceGoodStyle.Render(m.toast)
}

func (m Model) setPageNotification(lines []string, width, height int, fallback string) {
	if height < 3 {
		return
	}
	start := height - 3
	setLine(lines, start, actionLine(width, ""))
	setLine(lines, start+1, actionLine(width, surfaceFill("  ")+surfaceSectionStyle.Render("STATUS")))
	message := surfaceMutedStyle.Render(fallback)
	if m.toast != "" {
		message = m.surfaceNotificationView()
	}
	setLine(lines, start+2, actionLine(width, surfaceFill("  ")+message))
}

func (m Model) modalBodyView(width, height int) string {
	modal := modalRect(m.width, m.height)
	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		m.modalView(modal.width, modal.height),
		lipgloss.WithWhitespaceBackground(base),
	)
}

func (m Model) modalView(width, height int) string {
	borderColor := lime
	title := lipgloss.NewStyle().Bold(true).Foreground(lime).Background(surface).Render(m.modalTitle)
	if m.modalError {
		borderColor = red
		title = surfaceErrorStyle.Render(m.modalTitle)
	}
	border := lipgloss.NewStyle().Foreground(borderColor).Background(surface)
	plain := lipgloss.NewStyle().Foreground(text).Background(surface)
	message := wrapPlainText(m.modalMessage, max(1, width-6), 2)
	for len(message) < 2 {
		message = append(message, "")
	}
	buttonLabel := "[ Enter / click to confirm ]"
	button := lipgloss.NewStyle().Bold(true).Foreground(base).Background(green).Render(buttonLabel)
	buttonPad := max(0, (width-2-lipgloss.Width(button))/2)
	lines := []string{
		border.Render("╭" + strings.Repeat("─", max(0, width-2)) + "╮"),
		modalLine(width, "", border),
		modalLine(width, surfaceFill("  ")+title, border),
		modalLine(width, "", border),
		modalLine(width, surfaceFill("  ")+plain.Render(message[0]), border),
		modalLine(width, surfaceFill("  ")+plain.Render(message[1]), border),
		modalLine(width, "", border),
		modalLine(width, surfaceFill(strings.Repeat(" ", buttonPad))+button, border),
		modalLine(width, "", border),
		border.Render("╰" + strings.Repeat("─", max(0, width-2)) + "╯"),
	}
	if height < len(lines) {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func (m Model) logPositionView() string {
	if m.logOffset == 0 {
		return mutedStyle.Render("latest")
	}
	return keyStyle.Render(fmt.Sprintf("history +%d", m.logOffset))
}

func (m Model) logView(line logLine) string {
	prefix := mutedStyle.Render("  · info ")
	if line.level == "warn" {
		prefix = keyStyle.Render("  + warn ")
	} else if line.level == "error" {
		prefix = errorStyle.Render("  ! error") + " "
	}
	return prefix + lipgloss.NewStyle().Foreground(text).Render(normalizeLogMessage(line.message))
}

func normalizeLogMessage(message string) string {
	return strings.Join(strings.Fields(ansi.Strip(message)), " ")
}

func (m Model) localAddress() string {
	port := m.options.LocalPort
	if port == 0 {
		port = 7788
	}
	if m.status.LocalPort != 0 {
		port = m.status.LocalPort
	}
	return fmt.Sprintf("%s:%d", m.status.ListenAddr, port)
}

func panelLines(width, height int, lines []string, background lipgloss.Color) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	output := make([]string, height)
	for i := range height {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		output[i] = paintLine(width, line, background, text)
	}
	return strings.Join(output, "\n")
}

func barLine(width int, content string, background lipgloss.Color) string {
	return paintLine(width, content, background, text)
}

func paintLine(width int, content string, background, foreground lipgloss.Color) string {
	if width <= 0 {
		return ""
	}
	content = ansi.Truncate(content, width, "")
	prefix, suffix := styleEnvelope(background, foreground)
	if prefix != "" {
		content = strings.ReplaceAll(content, "\x1b[0m", "\x1b[0m"+prefix)
	}
	padding := max(0, width-lipgloss.Width(content))
	return prefix + content + strings.Repeat(" ", padding) + suffix
}

func styleEnvelope(background, foreground lipgloss.Color) (string, string) {
	const marker = "__SGFG_STYLE_MARKER__"
	rendered := lipgloss.NewStyle().Foreground(foreground).Background(background).Render(marker)
	index := strings.Index(rendered, marker)
	if index < 0 {
		return "", ""
	}
	return rendered[:index], rendered[index+len(marker):]
}

func actionLine(width int, content string) string {
	line := surfaceAccentStyle.Render("▌") + content
	remaining := max(0, width-lipgloss.Width(line))
	return line + surfaceFill(strings.Repeat(" ", remaining))
}

func surfaceFill(value string) string {
	return lipgloss.NewStyle().Foreground(text).Background(surface).Render(value)
}

func modalLine(width int, content string, border lipgloss.Style) string {
	innerWidth := max(0, width-2)
	remaining := max(0, innerWidth-lipgloss.Width(content))
	return border.Render("│") + content + surfaceFill(strings.Repeat(" ", remaining)) + border.Render("│")
}

func wrapPlainText(value string, width, maxLines int) []string {
	var lines []string
	var current strings.Builder
	currentWidth := 0
	for _, r := range value {
		runeWidth := lipgloss.Width(string(r))
		if currentWidth+runeWidth > width && current.Len() > 0 {
			lines = append(lines, current.String())
			if len(lines) == maxLines {
				return lines
			}
			current.Reset()
			currentWidth = 0
		}
		current.WriteRune(r)
		currentWidth += runeWidth
	}
	if current.Len() > 0 && len(lines) < maxLines {
		lines = append(lines, current.String())
	}
	return lines
}

func setLine(lines []string, index int, value string) {
	if index >= 0 && index < len(lines) {
		lines[index] = value
	}
}

func sidebarValue(label, value string) string {
	return "  " + mutedStyle.Render(label+": ") + lipgloss.NewStyle().Foreground(text).Render(value)
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func onOff(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func toggle(enabled bool) string {
	if enabled {
		return goodStyle.Render("[on]")
	}
	return mutedStyle.Render("[off]")
}
