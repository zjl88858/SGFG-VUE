package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestPaintLineReappliesBackgroundAfterNestedReset(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(previous)

	content := "  " + mutedStyle.Render("State: ") + lipgloss.NewStyle().Foreground(text).Render("idle")
	line := paintLine(24, content, base, text)
	prefix, _ := styleEnvelope(base, text)
	if !strings.Contains(line, "\x1b[0m"+prefix) {
		t.Fatalf("panel style was not restored after reset: %q", line)
	}
	if got := lipgloss.Width(line); got != 24 {
		t.Fatalf("painted width=%d", got)
	}
}

func TestPaintLineTruncatesWithoutWrapping(t *testing.T) {
	line := paintLine(20, strings.Repeat("long log ", 20), base, text)
	if lipgloss.Width(line) != 20 || lipgloss.Height(line) != 1 {
		t.Fatalf("painted line has dimensions %dx%d", lipgloss.Width(line), lipgloss.Height(line))
	}
}
