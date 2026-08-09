package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"sgfg-tui/internal/proxy"
)

type testSystemProxy struct{}

func (*testSystemProxy) Available() (bool, string) { return true, "test" }
func (*testSystemProxy) Enable(string) error       { return nil }
func (*testSystemProxy) Restore() error            { return nil }

func TestViewAtCommonTerminalSizes(t *testing.T) {
	manager := proxy.NewManager(&testSystemProxy{})
	model := NewModel(manager, make(chan any))
	for _, size := range []struct{ width, height int }{{60, 24}, {100, 30}, {140, 40}} {
		updated, _ := model.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
		model = updated.(Model)
		if output := model.View(); output == "" {
			t.Fatalf("empty view at %dx%d", size.width, size.height)
		} else {
			if got := lipgloss.Height(output); got != size.height {
				t.Fatalf("height at %dx%d: got %d", size.width, size.height, got)
			}
			for lineNumber, line := range strings.Split(output, "\n") {
				if got := lipgloss.Width(line); got > size.width {
					t.Fatalf("line %d at %dx%d is %d columns", lineNumber, size.width, size.height, got)
				}
			}
		}
	}
}

func TestAllPagesRender(t *testing.T) {
	manager := proxy.NewManager(&testSystemProxy{})
	model := NewModel(manager, make(chan any))
	model.width, model.height = 100, 30
	for _, current := range []page{homePage, optionsPage, relayPage} {
		model.page = current
		if output := model.View(); output == "" {
			t.Fatalf("empty page %d", current)
		}
	}
}

func TestMouseTabsAndTransportSelection(t *testing.T) {
	manager := proxy.NewManager(&testSystemProxy{})
	model := NewModel(manager, make(chan any))
	model.width, model.height = 120, 30
	layout := calculateLayout(model.width, model.height)

	model.handleMouse(tea.MouseMsg(tea.MouseEvent{
		X: layout.tabs[1].x, Y: layout.tabs[1].y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}))
	if model.page != optionsPage {
		t.Fatalf("tab click selected page %d", model.page)
	}

	model.page = homePage
	command := model.handleMouse(tea.MouseMsg(tea.MouseEvent{
		X: layout.transports[1].x + 2, Y: layout.transports[1].y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}))
	if command == nil || model.active != "hysteria2" || !model.busy {
		t.Fatalf("transport click was not handled: active=%q busy=%v", model.active, model.busy)
	}
}

func TestMouseWheelScrollsActivity(t *testing.T) {
	manager := proxy.NewManager(&testSystemProxy{})
	model := NewModel(manager, make(chan any))
	model.width, model.height = 120, 30
	for i := range 20 {
		model.logs = append(model.logs, logLine{level: "info", message: string(rune('a' + i))})
	}
	layout := calculateLayout(model.width, model.height)
	x, y := layout.logs.x+1, layout.logs.y+1

	model.handleMouse(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp}))
	if model.logOffset != 3 {
		t.Fatalf("wheel up offset=%d", model.logOffset)
	}
	model.handleMouse(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}))
	if model.logOffset != 0 {
		t.Fatalf("wheel down offset=%d", model.logOffset)
	}
}

func TestMouseOptionToggle(t *testing.T) {
	manager := proxy.NewManager(&testSystemProxy{})
	model := NewModel(manager, make(chan any))
	model.width, model.height, model.page = 120, 30, optionsPage
	layout := calculateLayout(model.width, model.height)
	command := model.handleMouse(tea.MouseMsg(tea.MouseEvent{
		X: layout.options[1].x + 2, Y: layout.options[1].y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}))
	if command == nil || model.optionsCursor != 1 {
		t.Fatal("option click did not produce an update command")
	}
	message := command()
	result, ok := message.(operationDoneMsg)
	if !ok {
		t.Fatalf("unexpected command result %T", message)
	}
	if result.err != nil || !result.options.AllowLAN {
		t.Fatalf("option was not toggled: %+v, err=%v", result.options, result.err)
	}
}

func TestLayoutSwitchesSidebarAtBreakpoint(t *testing.T) {
	if calculateLayout(95, 30).wide {
		t.Fatal("95 columns should use the compact layout")
	}
	layout := calculateLayout(96, 30)
	if !layout.wide || layout.sidebar.width == 0 || layout.main.width+layout.sidebar.width+2 != 96 {
		t.Fatalf("invalid wide layout: %+v", layout)
	}
}

func TestErrorUsesInTUIModal(t *testing.T) {
	manager := proxy.NewManager(&testSystemProxy{})
	model := NewModel(manager, make(chan any))
	model.width, model.height = 120, 30
	model.showToast("test notification", true)

	if !model.modalOpen || !strings.Contains(model.View(), "test notification") {
		t.Fatal("error did not open the in-TUI modal")
	}
	if strings.Contains(model.footerView(), "test notification") {
		t.Fatal("notification leaked into the footer")
	}
	if got := lipgloss.Height(model.footerView()); got != 1 {
		t.Fatalf("footer height=%d", got)
	}
	model.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if model.modalOpen {
		t.Fatal("Enter did not close the modal")
	}

	model.showToast("click to close", true)
	button := modalButtonRect(model.width, model.height)
	model.handleMouse(tea.MouseMsg(tea.MouseEvent{
		X: button.x + button.width/2, Y: button.y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}))
	if model.modalOpen {
		t.Fatal("mouse click did not close the modal")
	}
}

func TestActionLineKeepsExactPanelWidth(t *testing.T) {
	line := actionLine(72, surfaceFill("  ")+surfaceAccentStyle.Render("Connect")+surfaceMutedStyle.Render("  ·  click"))
	if got := lipgloss.Width(line); got != 72 {
		t.Fatalf("action line width=%d", got)
	}
}

func TestManyLogsDoNotMoveConnectActionOrHitTarget(t *testing.T) {
	manager := proxy.NewManager(&testSystemProxy{})
	model := NewModel(manager, make(chan any))
	model.width, model.height = 120, 30
	for i := range 60 {
		model.logs = append(model.logs, logLine{
			level:   "info",
			message: strings.Repeat("very long activity entry ", 20) + string(rune('a'+i%26)),
		})
	}
	layout := calculateLayout(model.width, model.height)
	output := model.View()
	if got := lipgloss.Height(output); got != model.height {
		t.Fatalf("render height=%d, want %d", got, model.height)
	}
	actionRow := findRenderedRow(output, "Connect")
	if want := layout.connect.y + 1; actionRow != want {
		t.Fatalf("Connect rendered at row %d, hit target is row %d", actionRow, want)
	}

	model.handleMouse(tea.MouseMsg(tea.MouseEvent{
		X: layout.connect.x + 4, Y: layout.connect.y + 1,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}))
	if !model.modalOpen || model.modalMessage != "请先选择传输方式" {
		t.Fatal("visual Connect row did not activate the connection action")
	}
}

func TestMultilineLogCannotChangeScreenHeight(t *testing.T) {
	manager := proxy.NewManager(&testSystemProxy{})
	model := NewModel(manager, make(chan any))
	model.width, model.height = 120, 30
	model.logs = []logLine{{level: "warn", message: "first line\r\nsecond line\nthird line"}}
	if got := lipgloss.Height(model.View()); got != model.height {
		t.Fatalf("multiline log changed screen height to %d", got)
	}
}

func findRenderedRow(output, value string) int {
	for row, line := range strings.Split(ansi.Strip(output), "\n") {
		if strings.Contains(line, value) {
			return row
		}
	}
	return -1
}
