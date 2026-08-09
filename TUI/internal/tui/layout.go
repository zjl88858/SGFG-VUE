package tui

import "github.com/charmbracelet/lipgloss"

const (
	headerHeight = 3
	footerHeight = 1
	wideAt       = 96
)

type rect struct {
	x      int
	y      int
	width  int
	height int
}

func (r rect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.width && y >= r.y && y < r.y+r.height
}

type screenLayout struct {
	body       rect
	main       rect
	sidebar    rect
	tabs       [3]rect
	transports [3]rect
	connect    rect
	logs       rect
	options    [4]rect
	relay      [4]rect
	wide       bool
}

func modalRect(width, height int) rect {
	modalWidth := min(68, max(40, width-6))
	modalHeight := min(10, max(8, height-headerHeight-footerHeight))
	return rect{
		x:      max(0, (width-modalWidth)/2),
		y:      headerHeight + max(0, (height-headerHeight-footerHeight-modalHeight)/2),
		width:  modalWidth,
		height: modalHeight,
	}
}

func modalButtonRect(width, height int) rect {
	modal := modalRect(width, height)
	buttonWidth := min(28, modal.width-4)
	return rect{
		x:      modal.x + (modal.width-buttonWidth)/2,
		y:      modal.y + modal.height - 3,
		width:  buttonWidth,
		height: 1,
	}
}

func calculateLayout(width, height int) screenLayout {
	bodyHeight := max(1, height-headerHeight-footerHeight)
	layout := screenLayout{
		body: rect{x: 0, y: headerHeight, width: width, height: bodyHeight},
		main: rect{x: 0, y: headerHeight, width: width, height: bodyHeight},
	}

	tabX := 2
	for i, label := range []string{" Home ", " Options ", " SS Relay "} {
		tabWidth := lipgloss.Width(label)
		layout.tabs[i] = rect{x: tabX, y: 1, width: tabWidth, height: 1}
		tabX += tabWidth + 1
	}

	if width >= wideAt {
		layout.wide = true
		sidebarWidth := min(38, max(30, width/3))
		layout.main.width = width - sidebarWidth - 2
		layout.sidebar = rect{
			x:      layout.main.width + 2,
			y:      headerHeight,
			width:  sidebarWidth,
			height: bodyHeight,
		}
	}

	for i := range 3 {
		layout.transports[i] = rect{x: layout.main.x, y: headerHeight + 3 + i*3, width: layout.main.width, height: 2}
	}
	connectY := headerHeight + max(0, bodyHeight-3)
	layout.connect = rect{x: layout.main.x, y: connectY, width: layout.main.width, height: min(3, bodyHeight)}
	logY := headerHeight + 12
	layout.logs = rect{
		x:      layout.main.x,
		y:      logY,
		width:  layout.main.width,
		height: max(0, connectY-logY),
	}
	for i := range 4 {
		y := headerHeight + 3 + i*3
		layout.options[i] = rect{x: layout.main.x, y: y, width: layout.main.width, height: 2}
		layout.relay[i] = layout.options[i]
	}
	return layout
}
