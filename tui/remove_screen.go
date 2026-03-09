package tui

import (
	"fmt"
	"strings"

	"codesnppet.dev/ifmproxy/relay"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type RemoveClientsScreen struct {
	cursor   int
	selected map[string]struct{}
}

func NewRemoveClientsScreen() *RemoveClientsScreen {
	return &RemoveClientsScreen{}
}

func (s *RemoveClientsScreen) Init(app *Model) tea.Cmd {
	s.cursor = 0
	s.selected = make(map[string]struct{})
	return nil
}

func (s *RemoveClientsScreen) Update(app *Model, msg tea.Msg) tea.Cmd {
	n := len(app.SortedClients)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case " ":
			if n == 0 {
				break
			}
			key := app.SortedClients[s.cursor].Addr.String()
			if _, ok := s.selected[key]; ok {
				delete(s.selected, key)
			} else {
				s.selected[key] = struct{}{}
			}
		case "up", "k":
			if n == 0 {
				break
			}
			s.cursor--
			if s.cursor < 0 {
				s.cursor = n - 1
			}
		case "down", "j":
			if n == 0 {
				break
			}
			s.cursor = (s.cursor + 1) % n
		case "enter":
			if len(s.selected) > 0 {
				app.RemoveClients(s.selected)
			}
			return app.ChangeScreen(SCREEN_MAIN)
		case "esc", "q":
			return app.ChangeScreen(SCREEN_MAIN)
		}
	}
	return nil
}

func (s *RemoveClientsScreen) View(app *Model, snap *relay.RelaySnapshot) string {
	bw := app.BoxWidth()

	if len(snap.Clients) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			subtleStyle.Render("  no clients to remove"),
			"",
			subtleStyle.Render("  Press ESC or Q to go back"),
		)
	}

	rows := make([]string, 0, len(app.SortedClients))
	for i, c := range app.SortedClients {
		addr := c.Addr.String()
		cursor := "  "
		if i == s.cursor {
			cursor = "> "
		}

		check := "[ ]"
		if _, ok := s.selected[addr]; ok {
			check = greenStyle.Render("[x]")
		}

		rows = append(rows, fmt.Sprintf(
			"%s %s %s %s %s %s",
			cursor,
			check,
			boldStyle.Render(addr),
			subtleStyle.Render(fmt.Sprintf("rx:%d", c.Stats.Received)),
			subtleStyle.Render(fmt.Sprintf("tx:%d", c.Stats.Sent)),
			subtleStyle.Render(RenderTimeAgo(c.Stats.LastPacketAt)),
		))
	}

	hint := subtleStyle.Render("  ↑/↓: move  space: toggle  enter: confirm  esc: cancel")

	return sectionBox.Width(bw).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			sectionTitle.Render("Remove Clients"),
			strings.Join(rows, "\n"),
			"",
			hint,
		),
	)
}
