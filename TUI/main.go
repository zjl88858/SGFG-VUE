package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"sgfg-tui/internal/proxy"
	"sgfg-tui/internal/sysproxy"
	"sgfg-tui/internal/tui"
)

func main() {
	systemProxy := sysproxy.New()
	manager := proxy.NewManager(systemProxy)
	defer manager.Shutdown()

	events := make(chan any, 128)
	manager.SetEmitter(func(event any) {
		select {
		case events <- event:
		default:
		}
	})

	program := tea.NewProgram(
		tui.NewModel(manager, events),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		<-signals
		program.Send(tui.QuitMsg{})
	}()

	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "sgfg-tui:", err)
		os.Exit(1)
	}
}
