package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ScreenId int

const (
	SCREEN_MAIN ScreenId = iota
	SCREEN_STATS
)

// RelayUpdatedMsg is sent whenever the relay signals that its
// state has changed. The TUI re-reads the snapshot on receipt.
type RelayUpdatedMsg struct{}

// errMsg is a TUI-internal message for startup errors that occur
// before the relay is running.
type errMsg struct{ err error }

type Model struct {
	relay *Relay

	screen ScreenId
	ci     textinput.Model
	err    error
	width  int
	height int
}

type AppConfig struct {
	Remote string `json:"remote"`
}

const appName = "ifm-relay"

const (
	COLOR_WHITE    = lipgloss.Color("#F0F0F0")
	COLOR_GRAY     = lipgloss.Color("#7A7A7A")
	COLOR_DARKGRAY = lipgloss.Color("#444444")
	COLOR_BLACK    = lipgloss.Color("#111111")
	COLOR_PURPLE   = lipgloss.Color("#6C6CFF")
	COLOR_GREEN    = lipgloss.Color("#00D3A7")
	COLOR_ERROR    = lipgloss.Color("#FF6C6C")
	COLOR_YELLOW   = lipgloss.Color("#FFD75F")
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(COLOR_WHITE).
			Background(COLOR_PURPLE).
			Padding(0, 1)
	boldStyle   = lipgloss.NewStyle().Bold(true)
	subtleStyle = lipgloss.NewStyle().
			Foreground(COLOR_GRAY)
	sectionTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(COLOR_GRAY)
	sectionBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(COLOR_DARKGRAY).
			Padding(0, 1)
	footerStyle = lipgloss.NewStyle().
			Foreground(COLOR_WHITE).
			Background(COLOR_DARKGRAY).
			Padding(0, 1)
	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(COLOR_ERROR)
	chipStyle = lipgloss.NewStyle().
			Foreground(COLOR_GREEN).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(COLOR_DARKGRAY).
			Padding(0, 1).
			MarginRight(1)

	redStyle    = lipgloss.NewStyle().Foreground(COLOR_ERROR)
	yellowStyle = lipgloss.NewStyle().Foreground(COLOR_YELLOW)
	greenStyle  = lipgloss.NewStyle().Foreground(COLOR_GREEN)
)

func initialModel(lport int) Model {
	input := textinput.New()
	input.Placeholder = "type ? for commands"
	input.Focus()
	input.CharLimit = 156
	input.Width = 40
	input.ShowSuggestions = true
	input.SetSuggestions([]string{"connect <ip>", "stats", "quit"})

	appCfg, _ := loadAppConfig()

	remote := ""
	if appCfg.Remote != "" {
		remote = fmt.Sprintf("%s", appCfg.Remote)
	}
	cfg := Cfg{listen: fmt.Sprintf("0.0.0.0:%d", lport), remote: remote}
	relay := NewRelay(cfg)

	return Model{
		relay:  relay,
		ci:     input,
		screen: SCREEN_MAIN,
	}
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		// Fallback
		home, herr := os.UserHomeDir()
		if herr != nil || home == "" {
			if err != nil {
				return "", err
			}
			return "", fmt.Errorf("cannot determine config directory")
		}
		dir = filepath.Join(home, ".config")
	}
	appDir := filepath.Join(dir, appName)
	if mkerr := os.MkdirAll(appDir, 0o755); mkerr != nil {
		return "", mkerr
	}
	return filepath.Join(appDir, "config.json"), nil
}

func loadAppConfig() (AppConfig, error) {
	var cfg AppConfig
	p, err := configPath()
	if err != nil {
		return cfg, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func saveAppConfig(cfg AppConfig) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func (m Model) Init() tea.Cmd {
	start := func() tea.Msg {
		if err := m.relay.Start(); err != nil {
			return errMsg{err: err}
		}
		return nil
	}
	return tea.Batch(
		textinput.Blink,
		start,
		waitRelaySignal(m.relay),
	)
}

func waitRelaySignal(r *Relay) tea.Cmd {
	return func() tea.Msg {
		_, ok := <-r.NotifyChan()
		if !ok {
			return nil
		}
		return RelayUpdatedMsg{}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		inputWidth := msg.Width - sectionBox.GetHorizontalFrameSize() - 2
		if inputWidth < 20 {
			inputWidth = 20
		}
		m.ci.Width = inputWidth

	case RelayUpdatedMsg:
		snap := m.relay.Snapshot()
		if snap.LastErr != nil {
			m.err = snap.LastErr
		}
		return m, waitRelaySignal(m.relay)

	case errMsg:
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.relay.Stop()
			return m, tea.Quit
		case "enter":
			if !m.ci.Focused() {
				break
			}
			m.err = nil
			parts := strings.Split(strings.TrimSpace(m.ci.Value()), " ")
			if len(parts) == 0 {
				break
			}
			val := parts[0]
			args := parts[1:]

			if isIPAddr(val) {
				m.connectTo(val)
				m.ci.SetValue("")
				break
			}

			switch val {
			case "?":
				break
			case "stats":
				m.screen = SCREEN_STATS
			case "quit", "exit":
				m.relay.Stop()
				return m, tea.Quit
			case "connect":
				if len(args) > 0 {
					m.connectTo(args[0])
				}
			}
			m.ci.SetValue("")
		case "esc", "q":
			if m.screen == SCREEN_STATS {
				m.screen = SCREEN_MAIN
				return m, nil
			}
		}
	}

	m.ci, cmd = m.ci.Update(msg)
	return m, cmd
}

func (m *Model) connectTo(ip string) {
	if _, err := net.ResolveIPAddr("ip4", ip); err != nil {
		m.err = err
		return
	}
	_ = saveAppConfig(AppConfig{Remote: ip})
	if err := m.relay.SetRemote(ip); err != nil {
		m.err = err
	}
}

func isIPAddr(s string) bool {
	_, err := net.ResolveIPAddr("ip4", s)
	return err == nil
}

// contentWidth returns the usable terminal width, defaulting to 80.
func (m Model) contentWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 80
}

// boxWidth returns the inner content width for bordered sections.
func (m Model) boxWidth() int {
	return m.contentWidth() - sectionBox.GetHorizontalFrameSize()
}

func (m Model) View() string {
	snap := m.relay.Snapshot()
	w := m.contentWidth()

	title := titleStyle.Width(w - titleStyle.GetHorizontalFrameSize()).Render("iFacialMocap Relay")

	var content string
	switch m.screen {
	case SCREEN_MAIN:
		content = m.viewMain(snap)
	case SCREEN_STATS:
		content = m.viewStats(snap)
	default:
		content = m.viewMain(snap)
	}

	out := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		m.footer(),
	)
	return out
}

func (m Model) viewMain(snap RelaySnapshot) string {
	bw := m.boxWidth()

	status := renderRelayStatusInline(snap)
	connectionBox := sectionBox.Width(bw).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			sectionTitle.Render("Connection"),
			status,
		),
	)

	clients := renderClientsInline(snap.Clients)
	clientsBox := sectionBox.Width(bw).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			sectionTitle.Render("Clients"),
			clients,
		),
	)

	var help string
	showHelp := strings.TrimSpace(m.ci.Value()) == "?"
	if showHelp {
		help = sectionBox.Width(bw).Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				sectionTitle.Render("Commands"),
				subtleStyle.Render("  connect <ip>  set remote and start listener"),
				subtleStyle.Render("  stats         view last packet and counters"),
				subtleStyle.Render("  quit          exit"),
			),
		)
	}

	prompt := m.ci.View()

	sections := []string{
		connectionBox,
		"",
		clientsBox,
	}
	if help != "" {
		sections = append(sections, "", help)
	}
	sections = append(sections, "", prompt)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func renderRelayStatusInline(snap RelaySnapshot) string {

	relayLine := fmt.Sprintf("  Relay         %s  %s",
		boldStyle.Render(snap.ListenAddr),
		renderDotStatus(snap.Status),
	)

	upstreamAddr := snap.RemoteAddr
	if upstreamAddr == "" {
		upstreamAddr = "<not set>"
	}
	upstreamStatus := STATUS_STOPPED
	if snap.Upstream != nil {
		upstreamStatus = snap.Upstream.status
	}

	if upstreamStatus == STATUS_GOOD && snap.Upstream != nil {
		elapsed := time.Since(snap.Upstream.stats.lastPacketAt)
		if elapsed > time.Second {
			upstreamStatus = STATUS_WAITING
		}
	}
	upstreamLine := fmt.Sprintf("  iFacialMocap  %s  %s",
		boldStyle.Render(upstreamAddr),
		renderDotStatus(upstreamStatus),
	)

	if snap.Upstream != nil && upstreamStatus != STATUS_STOPPED {
		upstreamLine += subtleStyle.Render(fmt.Sprintf("  %d pkts, %s",
			snap.Upstream.stats.received, subtleStyle.Render(renderTimeAgoInline(snap.Upstream.stats.lastPacketAt))))
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		relayLine,
		upstreamLine,
	)
}

const DOT = "●"

func renderDotStatus(status Status) string {
	switch status {
	case STATUS_STOPPED:
		return redStyle.Render(DOT + " Stopped")
	case STATUS_WAITING:
		return yellowStyle.Render(DOT + " Waiting")
	case STATUS_GOOD:
		return greenStyle.Render(DOT + " Connected")
	}
	return ""
}

func renderClientsInline(clients map[string]*Client) string {
	if len(clients) == 0 {
		return subtleStyle.Render("  no clients connected")
	}

	addrs := sortedClientKeys(clients)

	var b strings.Builder
	for _, k := range addrs {
		c := clients[k]
		fmt.Fprintf(&b,
			"  %s  %s %s %s\n",
			boldStyle.Render(k),
			subtleStyle.Render(fmt.Sprintf("rx:%d", c.stats.received)),
			subtleStyle.Render(fmt.Sprintf("tx:%d", c.stats.sent)),
			subtleStyle.Render(renderTimeAgoInline(c.stats.lastPacketAt)),
		)
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderTimeAgoInline(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	ago := time.Since(t).Truncate(time.Second)
	return fmt.Sprintf("%s ago", ago)
}

func (m Model) viewStats(snap RelaySnapshot) string {
	bw := m.boxWidth()

	var upstreamChips string
	if snap.Upstream != nil {
		upstreamChips = lipgloss.JoinHorizontal(lipgloss.Top,
			chipStyle.Render(fmt.Sprintf("rx: %d", snap.Upstream.stats.received)),
			chipStyle.Render(renderTimeAgoInline(snap.Upstream.stats.lastPacketAt)),
		)
	} else {
		upstreamChips = subtleStyle.Render("no upstream configured")
	}

	preview := subtleStyle.Render("<no data>")
	if snap.Upstream != nil && len(snap.Upstream.stats.lastPacket) > 0 {
		preview = safePreview(snap.Upstream.stats.lastPacket, 256)
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

	addrs := sortedClientKeys(snap.Clients)
	var clientRows []string
	var totalRelayed int
	for _, k := range addrs {
		c := snap.Clients[k]
		totalRelayed += c.stats.sent
		clientRows = append(clientRows,
			lipgloss.JoinHorizontal(lipgloss.Top,
				boldStyle.Render(k),
				chipStyle.Render(fmt.Sprintf("rx: %d", c.stats.received)),
				chipStyle.Render(fmt.Sprintf("tx: %d", c.stats.sent)),
				chipStyle.Render(renderTimeAgoInline(c.stats.lastPacketAt)),
			),
		)
	}
	var clientContent string
	if len(clientRows) == 0 {
		clientContent = subtleStyle.Render("  no clients connected")
	} else {
		clientContent = strings.Join(clientRows, "\n")
	}

	var totalLine string
	if totalRelayed > 0 {
		totalLine = "\n" + chipStyle.Render(fmt.Sprintf("total relayed: %d", totalRelayed))
	}

	clientsBox := sectionBox.Width(bw).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			sectionTitle.Render("Clients"),
			clientContent+totalLine,
		),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		upstreamBox,
		"",
		clientsBox,
		"",
		subtleStyle.Render("  Press ESC or Q to go back"),
	)
}

func safePreview(b []byte, max int) string {
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

func (m Model) footer() string {
	if m.err == nil {
		return ""
	}
	w := m.contentWidth()
	return footerStyle.Width(w - footerStyle.GetHorizontalFrameSize()).Render(
		errorStyle.Render(m.err.Error()),
	)
}

func sortedClientKeys(clients map[string]*Client) []string {
	addrs := make([]string, 0, len(clients))
	for k := range clients {
		addrs = append(addrs, k)
	}
	sort.Strings(addrs)
	return addrs
}
