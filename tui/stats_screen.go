package tui

import (
	"fmt"

	"codesnppet.dev/ifmproxy/network"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type StatsScreen struct{}

func NewStatsScreen() *StatsScreen {
	return &StatsScreen{}
}

func (s *StatsScreen) Init(app *Model) tea.Cmd {
	return nil
}

func (s *StatsScreen) Update(app *Model, msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return app.ChangeScreen(SCREEN_MAIN)
		}
	}
	return nil
}

func (s *StatsScreen) View(app *Model, snap *network.RelaySnapshot) string {
	bw := app.BoxWidth()

	var upstreamChips string
	if snap.Upstream != nil {
		upstreamChips = lipgloss.JoinHorizontal(lipgloss.Top,
			chipStyle.Render(fmt.Sprintf("rx: %d", snap.Upstream.Stats.Received)),
			chipStyle.Render(RenderTimeAgo(snap.Upstream.Stats.LastPacketAt)),
		)
	} else {
		upstreamChips = subtleStyle.Render("no upstream configured")
	}

	preview := subtleStyle.Render("<no data>")
	if snap.Upstream != nil && len(snap.Upstream.Stats.LastPacket) > 0 {
		preview = SafePreview(snap.Upstream.Stats.LastPacket, 256)
	}

	upstreamBox := sectionBox.Width(bw).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			sectionTitle.Render("Upstream"),
			upstreamChips,
			"",
			subtleStyle.Render("Last Packet:"),
			preview,
		),
	)

	clients := app.SortedClients
	var totalRelayed int
	var totalReceived int
	if snap.Upstream != nil {
		totalReceived = snap.Upstream.Stats.Received
	}
	for _, c := range clients {
		totalRelayed += c.Stats.Sent
		totalReceived += c.Stats.Received
	}

	totalsBox := sectionBox.Width(bw).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			sectionTitle.Render("Totals"),
			chipStyle.Render(fmt.Sprintf("rx: %d", totalReceived)),
			chipStyle.Render(fmt.Sprintf("tx: %d", totalRelayed)),
		),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		upstreamBox,
		"",
		totalsBox,
		"",
		subtleStyle.Render("  Press ESC or Q to go back"),
	)
}

func SafePreview(b []byte, max int) string {
	if len(b) > max {
		b = b[:max]
	}
	clean := make([]rune, 0, len(b))
	for _, by := range b {
		if by >= 32 && by <= 126 {
			clean = append(clean, rune(by))
		} else if by == '\n' || by == '\r' || by == '\t' {
			clean = append(clean, rune(by))
		} else {
			clean = append(clean, '·')
		}
	}
	s := string(clean)
	if len(b) == max {
		s += subtleStyle.Render(" ...")
	}
	return s
}
