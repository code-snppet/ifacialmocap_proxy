package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"codesnppet.dev/ifmproxy/logger"
	"codesnppet.dev/ifmproxy/network"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ScreenId int

const (
	SCREEN_MAIN ScreenId = iota
	SCREEN_STATS
	SCREEN_REMOVE_CLIENTS
	SCREEN_LOGS
)

const APP_NAME = "ifmproxy"
const APP_TITLE = "iFacialMocap Proxy"

type Screen interface {
	Init(app *Model) tea.Cmd
	Update(app *Model, msg tea.Msg) tea.Cmd
	View(app *Model, snap *network.RelaySnapshot) string
}

var screens map[ScreenId]func() Screen

func init() {
	screens = map[ScreenId]func() Screen{
		SCREEN_MAIN:           func() Screen { return NewMainScreen() },
		SCREEN_STATS:          func() Screen { return NewStatsScreen() },
		SCREEN_REMOVE_CLIENTS: func() Screen { return NewRemoveClientsScreen() },
		SCREEN_LOGS:           func() Screen { return NewLogsScreen() },
	}
}

type RelayUpdatedMsg struct{}

type errMsg struct{ err error }

type LogUpdatedMsg struct{}

type Model struct {
	AppCfg AppConfig
	Relay  *network.Relay

	screen Screen
	err    error
	logger *logger.Logger
	width  int
	height int

	Scanning       bool
	autoConnecting bool
	scanCancel     context.CancelFunc

	Snapshot      network.RelaySnapshot
	SortedClients []*network.Client
	AutoClients   []*network.Client
	ManualClients []*network.Client
}

func InitialModel(ipFlag string, portFlag int, logger *logger.Logger) Model {
	appCfg, _ := LoadAppConfig()

	remote := appCfg.Remote
	listen := ""
	defaultIp := "0.0.0.0"
	defaultPort := network.IFM_PORT
	if ipFlag != "" || portFlag > 0 {
		if ipFlag != "" {
			defaultIp = ipFlag
		}
		if portFlag > 0 {
			defaultPort = portFlag
		}
		listen = fmt.Sprintf("%s:%d", defaultIp, defaultPort)
	} else if appCfg.Listen != "" {
		listen = appCfg.Listen
	} else {
		listen = fmt.Sprintf("%s:%d", defaultIp, defaultPort)
	}
	cfg := network.Cfg{Listen: listen, Remote: remote}
	r := network.NewRelay(cfg, logger)

	if len(appCfg.ManualAddresses) > 0 {
		for _, addr := range appCfg.ManualAddresses {
			parts := strings.Split(addr, ":")
			if len(parts) < 2 {
				continue
			}
			port, err := strconv.Atoi(parts[1])
			if err != nil {
				continue
			}
			r.AddClient(parts[0], port)
		}
	}
	screen := screens[SCREEN_MAIN]()

	return Model{
		AppCfg:         appCfg,
		Relay:          r,
		screen:         screen,
		logger:         logger,
		autoConnecting: remote != "",
	}
}

func (m Model) Init() tea.Cmd {
	start := func() tea.Msg {
		if err := m.Relay.Start(); err != nil {
			return errMsg{err: err}
		}
		return RelayUpdatedMsg{}
	}
	screenInit := m.screen.Init(&m)
	cmds := []tea.Cmd{
		start,
		screenInit,
		waitRelaySignal(m.Relay),
		waitLogSignal(m.logger),
	}
	if m.autoConnecting {
		cmds = append(cmds, scheduleAutoConnectCheck())
	}
	return tea.Batch(cmds...)
}

func waitRelaySignal(r *network.Relay) tea.Cmd {
	return func() tea.Msg {
		_, ok := <-r.NotifyChan()
		if !ok {
			return nil
		}
		return RelayUpdatedMsg{}
	}
}

func waitLogSignal(logger *logger.Logger) tea.Cmd {
	return func() tea.Msg {
		_, ok := <-logger.NotifyChan()
		if !ok {
			return nil
		}
		return LogUpdatedMsg{}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case RelayUpdatedMsg:
		snap := m.Relay.Snapshot()
		m.Snapshot = snap
		if snap.LastErr != nil {
			m.err = snap.LastErr
		}
		var autoCount int
		m.SortedClients, autoCount = m.sortedClients(snap.Clients)
		m.AutoClients = m.SortedClients[:autoCount]
		m.ManualClients = m.SortedClients[autoCount:]
		if m.autoConnecting && m.Relay.IsUpstreamAlive() {
			m.autoConnecting = false
			m.logger.Info("Auto-connect: upstream is alive")
		}
		return m, waitRelaySignal(m.Relay)

	case LogUpdatedMsg:
		m.screen.Update(&m, msg)
		return m, waitLogSignal(m.logger)

	case ScanResultMsg:
		cmd := m.handleScanResult(msg)
		return m, cmd

	case AutoConnectTickMsg:
		cmd := m.handleAutoConnectTick()
		return m, cmd

	case errMsg:
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.Relay.Stop()
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	cmd = m.screen.Update(&m, msg)
	return m, cmd
}

func (m Model) View() string {
	w := m.ContentWidth()

	title := titleStyle.Width(w - titleStyle.GetHorizontalFrameSize()).Render("iFacialMocap Proxy")
	content := m.screen.View(&m, &m.Snapshot)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		m.footer(),
	)
}

func (m *Model) ChangeScreen(id ScreenId) tea.Cmd {
	newScreenFunc, ok := screens[id]
	if !ok {
		return nil
	}

	m.screen = newScreenFunc()
	return m.screen.Init(m)
}

func (m *Model) SetErr(err error) {
	m.err = err
}

func (m Model) ContentWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 80
}

func (m Model) BoxWidth() int {
	return m.ContentWidth() - sectionBox.GetHorizontalFrameSize()
}

func (m Model) footer() string {
	if m.err == nil {
		return ""
	}
	w := m.ContentWidth()
	return footerStyle.Width(w - footerStyle.GetHorizontalFrameSize()).Render(
		errorStyle.Render(m.err.Error()),
	)
}
