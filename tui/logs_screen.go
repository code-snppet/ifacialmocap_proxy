package tui

import (
	"fmt"
	"strings"

	"codesnppet.dev/ifmproxy/logger"
	"codesnppet.dev/ifmproxy/relay"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type LogsScreen struct {
	offset int
}

func NewLogsScreen() *LogsScreen {
	return &LogsScreen{}
}

func (s *LogsScreen) Init(app *Model) tea.Cmd {
	entries := app.logger.Entries()
	pageSize := s.visibleLines(app)
	pages := len(entries) / pageSize
	s.offset = (pages - 1) * pageSize
	if s.offset < 0 {
		s.offset = 0
	}
	return nil
}

func (s *LogsScreen) Update(app *Model, msg tea.Msg) tea.Cmd {
	n := len(app.logger.Entries())

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return app.ChangeScreen(SCREEN_MAIN)
		case "up", "k":
			if s.offset > 0 {
				s.offset--
			}
		case "down", "j":
			maxOffset := s.maxOffset(app)
			if s.offset < maxOffset {
				s.offset++
			}
		case "home", "g":
			s.offset = 0
		case "end", "G":
			s.offset = s.maxOffset(app)
		case "ctrl+u":
			visible := s.visibleLines(app)
			s.offset -= visible
			if s.offset < 0 {
				s.offset = 0
			}
		case "ctrl+d":
			visible := s.visibleLines(app)
			s.offset += visible
			maxOffset := s.maxOffset(app)
			if s.offset > maxOffset {
				s.offset = maxOffset
			}
		case "c":
			app.logger.Clear()
			s.offset = 0
		}
		_ = n
	}
	return nil
}

func (s *LogsScreen) View(app *Model, snap *relay.RelaySnapshot) string {
	bw := app.BoxWidth()
	entries := app.logger.Entries()

	if len(entries) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			subtleStyle.Render("  no log entries"),
			"",
			subtleStyle.Render("  Press ESC or Q to go back"),
		)
	}

	visible := s.visibleLines(app)
	end := s.offset + visible
	if end > len(entries) {
		end = len(entries)
	}
	start := s.offset
	if start > len(entries) {
		start = len(entries)
	}

	lines := entries[start:end]

	numbered := make([]string, len(lines))
	for i, line := range lines {
		numbered[i] = s.renderEntry(line)
	}

	scrollInfo := subtleStyle.Render(fmt.Sprintf(
		"  %d-%d of %d entries",
		start+1, end, len(entries),
	))

	hint := subtleStyle.Render("  j/k: scroll  ctrl+u/ctrl+d: page  g/G: top/bottom  c: clear  esc: back")

	return sectionBox.Width(bw).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			sectionTitle.Render("Logs"),
			strings.Join(numbered, "\n"),
			"",
			scrollInfo,
			hint,
		),
	)
}

func (s *LogsScreen) renderEntry(entry logger.Entry) string {
	style := subtleStyle
	if entry.Level == "DEBUG" {
		style = subtleStyle
	} else if entry.Level == "INFO" {
		style = bodyStyle
	} else if entry.Level == "WARNING" {
		style = yellowStyle
	} else if entry.Level == "ERROR" {
		style = redStyle
	}
	return style.Render(fmt.Sprintf("%s [%s] %s", entry.Time.Format("15:04:05"), entry.Level, entry.Message))
}

func (s *LogsScreen) visibleLines(app *Model) int {
	// Reserve lines for title, section box chrome, scroll info, hint, padding
	available := app.height - 10
	if available < 5 {
		available = 5
	}
	return available
}

func (s *LogsScreen) maxOffset(app *Model) int {
	visible := s.visibleLines(app)
	max := len(app.logger.Entries()) - visible
	if max < 0 {
		return 0
	}
	return max
}
