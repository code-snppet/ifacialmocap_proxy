package tui

import (
	"fmt"
	"strings"

	"codesnppet.dev/ifmproxy/logger"
	"codesnppet.dev/ifmproxy/network"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type LogsScreen struct {
	offset     int
	autoScroll bool
}

func NewLogsScreen() *LogsScreen {
	return &LogsScreen{}
}

func (s *LogsScreen) Init(app *Model) tea.Cmd {
	s.offset = s.maxOffset(app)
	s.autoScroll = true
	if s.offset < 0 {
		s.offset = 0
	}
	return nil
}

func (s *LogsScreen) Update(app *Model, msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case LogUpdatedMsg:
		if s.autoScroll {
			s.offset = s.maxOffset(app)
		}
	case tea.KeyMsg:
		if s.autoScroll {
			s.offset = s.maxOffset(app)
		}
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
		if s.offset >= s.maxOffset(app) {
			s.autoScroll = true
		} else {
			s.autoScroll = false
		}
	}
	return nil
}

func (s *LogsScreen) View(app *Model, snap *network.RelaySnapshot) string {
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

	autoScroll := ""
	if s.autoScroll {
		autoScroll = "  auto-scrolling"
	}
	scrollInfo := subtleStyle.Render(fmt.Sprintf(
		"  %d-%d of %d entries %s",
		start+1, end, len(entries),
		autoScroll,
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
