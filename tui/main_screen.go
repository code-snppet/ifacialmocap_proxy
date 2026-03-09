package tui

import (
	"fmt"
	"net"
	"strings"
	"time"

	"codesnppet.dev/ifmproxy/relay"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Command struct {
	trigger     string
	name        string
	description string
	action      func(app *Model, args []string) tea.Cmd
}

type MainScreen struct {
	ci       textinput.Model
	commands []Command
}

const DEFAULT_SCAN_SUBNET = "192.168.1.0/24"

func NewMainScreen() *MainScreen {
	ci := textinput.New()
	ci.Placeholder = "type ? for commands"
	ci.Focus()
	ci.CharLimit = 156
	ci.Width = 40
	ci.ShowSuggestions = true

	commands := []Command{
		{
			trigger:     "connect",
			name:        "connect <address>",
			description: "connect to an iFacialMocap device ip[:port] (default port: 49983)",
			action:      connectCommand,
		},
		{
			trigger:     "scan",
			name:        "scan <subnet> (default: 192.168.1.0/24)",
			description: "EXPERIMENTAL: scan for iFacialMocap devices in the subnet",
			action:      scanCommand,
		},
		{
			trigger:     "listen",
			name:        "listen <address>",
			description: "set relay address ip[:port] (default: 0.0.0.0:49983)",
			action:      listenCommand,
		},
		{
			trigger:     "add",
			name:        "add <address>",
			description: "add client address ip:port",
			action:      addCommand,
		},
		{
			trigger:     "remove",
			name:        "remove",
			description: "select and remove clients",
			action:      removeCommand,
		},
		{
			trigger:     "stats",
			name:        "stats",
			description: "view last packet and counters",
			action:      statsCommand,
		},
		{
			trigger:     "logs",
			name:        "logs",
			description: "view application logs",
			action:      logsCommand,
		},
		{
			trigger:     "quit",
			name:        "quit",
			description: "quit the program",
			action:      quitCommand,
		},
	}

	suggestions := make([]string, len(commands))
	for i, c := range commands {
		suggestions[i] = c.name
	}
	ci.SetSuggestions(suggestions)

	return &MainScreen{
		ci:       ci,
		commands: commands,
	}
}

func (s *MainScreen) Init(app *Model) tea.Cmd {
	s.ci.Focus()
	return textinput.Blink
}

func (s *MainScreen) Update(app *Model, msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		inputWidth := msg.Width - sectionBox.GetHorizontalFrameSize() - 2
		if inputWidth < 20 {
			inputWidth = 20
		}
		s.ci.Width = inputWidth

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if !s.ci.Focused() {
				break
			}
			parts := strings.Split(strings.TrimSpace(s.ci.Value()), " ")
			if len(parts) == 0 {
				break
			}
			val := parts[0]
			app.err = nil

			if ip := net.ParseIP(val); ip != nil {
				app.ConnectTo(val)
				s.ci.SetValue("")
				break
			}
			return s.runCommand(app, parts)
		}
	}

	s.ci, cmd = s.ci.Update(msg)
	return cmd
}

func (s *MainScreen) runCommand(app *Model, args []string) tea.Cmd {
	name := strings.ToLower(args[0])
	var cmdArgs []string
	if len(args) > 1 {
		cmdArgs = args[1:]
	}
	for _, c := range s.commands {
		if name == c.trigger {
			s.ci.SetValue("")
			return c.action(app, cmdArgs)
		}
	}
	return nil
}

func (s *MainScreen) View(app *Model, snap *relay.RelaySnapshot) string {
	bw := app.BoxWidth()

	status := s.renderRelayStatus(snap)
	connectionBox := sectionBox.Width(bw).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			sectionTitle.Render("Connection"),
			status,
		),
	)

	clientsBox := s.renderClientsBox(app, snap)

	var help string
	if strings.TrimSpace(s.ci.Value()) == "?" {
		helpContent := []string{sectionTitle.Render("Commands")}
		for _, c := range s.commands {
			helpContent = append(helpContent, subtleStyle.Render(
				fmt.Sprintf("  %-18s %s", c.name, c.description),
			))
		}
		help = sectionBox.Width(bw).Render(
			lipgloss.JoinVertical(lipgloss.Left, helpContent...),
		)
	}

	prompt := s.ci.View()

	sections := []string{connectionBox, "", clientsBox}
	if help != "" {
		sections = append(sections, "", help)
	}
	sections = append(sections, "", prompt)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (s *MainScreen) renderRelayStatus(snap *relay.RelaySnapshot) string {
	relayLine := fmt.Sprintf("  Relay         %s  %s",
		boldStyle.Render(snap.ListenAddr),
		RenderStatus(snap.Status),
	)

	upstreamAddr := snap.RemoteAddr
	if upstreamAddr == "" {
		upstreamAddr = "<not set>"
	}
	upstreamStatus := relay.STATUS_STOPPED
	if snap.Upstream != nil {
		upstreamStatus = snap.Upstream.Status
	}
	if upstreamStatus == relay.STATUS_GOOD && snap.Upstream != nil {
		if time.Since(snap.Upstream.Stats.LastPacketAt) > time.Second {
			upstreamStatus = relay.STATUS_WAITING
		}
	}
	scanning := ""
	if snap.Scanning {
		scanning = "Scanning..."
	}
	upstreamLine := fmt.Sprintf("  iFacialMocap  %s  %s  %s",
		boldStyle.Render(upstreamAddr),
		RenderStatus(upstreamStatus),
		scanning,
	)
	if snap.Upstream != nil && upstreamStatus != relay.STATUS_STOPPED {
		upstreamLine += subtleStyle.Render(fmt.Sprintf("  %d pkts, %s",
			snap.Upstream.Stats.Received,
			subtleStyle.Render(RenderTimeAgo(snap.Upstream.Stats.LastPacketAt)),
		))
	}

	return lipgloss.JoinVertical(lipgloss.Left, relayLine, upstreamLine)
}

func (s *MainScreen) renderClientsBox(app *Model, snap *relay.RelaySnapshot) string {
	if len(snap.Clients) == 0 {
		return subtleStyle.Render("  no clients connected")
	}

	rows := make([]string, 0, len(app.SortedClients))
	for _, c := range app.SortedClients {
		rows = append(rows, RenderClient(c))
	}

	bw := app.BoxWidth()
	return sectionBox.Width(bw).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			sectionTitle.Render("Autoconnected Clients"),
			strings.Join(rows[:len(app.AutoClients)], ""),
			sectionTitle.Render("Manual Clients"),
			strings.Join(rows[len(app.AutoClients):], ""),
		),
	)
}

func connectCommand(app *Model, args []string) tea.Cmd {
	if len(args) == 0 {
		return nil
	}
	app.ConnectTo(args[0])
	return nil
}

func scanCommand(app *Model, args []string) tea.Cmd {
	subnet := DEFAULT_SCAN_SUBNET
	if len(args) > 0 {
		subnet = args[0]
	}
	app.Scan(subnet)
	return nil
}

func listenCommand(app *Model, args []string) tea.Cmd {
	if len(args) == 0 {
		return nil
	}
	app.ListenTo(args[0])
	return nil
}

func addCommand(app *Model, args []string) tea.Cmd {
	if len(args) == 0 {
		return nil
	}
	app.AddClient(args[0])
	return nil
}

func removeCommand(app *Model, args []string) tea.Cmd {
	return app.ChangeScreen(SCREEN_REMOVE_CLIENTS)
}

func statsCommand(app *Model, args []string) tea.Cmd {
	return app.ChangeScreen(SCREEN_STATS)
}

func logsCommand(app *Model, args []string) tea.Cmd {
	return app.ChangeScreen(SCREEN_LOGS)
}

func quitCommand(app *Model, args []string) tea.Cmd {
	return tea.Quit
}
