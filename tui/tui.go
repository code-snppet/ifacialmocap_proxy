package tui

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"codesnppet.dev/ifmproxy/relay"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ScreenId int

const (
	SCREEN_MAIN ScreenId = iota
	SCREEN_STATS
	SCREEN_REMOVE_CLIENTS
)

type Screen interface {
	Init(app *Model) tea.Cmd
	Update(app *Model, msg tea.Msg) tea.Cmd
	View(app *Model, snap *relay.RelaySnapshot) string
}

var screens map[ScreenId]func() Screen

func init() {
	screens = map[ScreenId]func() Screen{
		SCREEN_MAIN:           func() Screen { return NewMainScreen() },
		SCREEN_STATS:          func() Screen { return NewStatsScreen() },
		SCREEN_REMOVE_CLIENTS: func() Screen { return NewRemoveClientsScreen() },
	}
}

type RelayUpdatedMsg struct{}

type errMsg struct{ err error }

type Model struct {
	AppCfg AppConfig
	Relay  *relay.Relay

	screen Screen
	err    error
	width  int
	height int

	Snapshot      relay.RelaySnapshot
	SortedClients []*relay.Client
	AutoClients   []*relay.Client
	ManualClients []*relay.Client
}

type AppConfig struct {
	Remote          string   `json:"remote"`
	Listen          string   `json:"listen"`
	ManualAddresses []string `json:"manual_addresses"`
}

const appName = "ifm-relay"

func InitialModel(ipFlag string, portFlag int) Model {
	appCfg, _ := loadAppConfig()

	remote := appCfg.Remote
	listen := ""
	defaultIp := "0.0.0.0"
	defaultPort := relay.IFM_PORT
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
	cfg := relay.Cfg{Listen: listen, Remote: remote}
	r := relay.NewRelay(cfg)

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
		AppCfg: appCfg,
		Relay:  r,
		screen: screen,
	}
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
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

func SaveAppConfig(cfg AppConfig) error {
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
		if err := m.Relay.Start(); err != nil {
			return errMsg{err: err}
		}
		return RelayUpdatedMsg{}
	}
	screenInit := m.screen.Init(&m)
	return tea.Batch(
		start,
		screenInit,
		waitRelaySignal(m.Relay),
	)
}

func waitRelaySignal(r *relay.Relay) tea.Cmd {
	return func() tea.Msg {
		_, ok := <-r.NotifyChan()
		if !ok {
			return nil
		}
		return RelayUpdatedMsg{}
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
		return m, waitRelaySignal(m.Relay)

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

	title := titleStyle.Width(w - titleStyle.GetHorizontalFrameSize()).Render("iFacialMocap Relay")
	content := m.screen.View(&m, &m.Snapshot)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		m.footer(),
	)
}

// ───────────────────── Model helpers ─────────────────────

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

func (m *Model) ConnectTo(addr string) {
	ip, port, err := ToHostPort(addr, relay.IFM_PORT)
	if err != nil {
		m.err = err
		return
	}
	m.AppCfg.Remote = addr
	_ = SaveAppConfig(m.AppCfg)
	if err := m.Relay.SetRemote(ip, port); err != nil {
		m.err = err
	}
}

func (m *Model) ListenTo(addr string) {
	ip, port, err := ToHostPort(addr)
	if err != nil {
		m.err = err
		return
	}
	m.AppCfg.Listen = addr
	_ = SaveAppConfig(m.AppCfg)
	if err := m.Relay.SetListen(ip, port); err != nil {
		m.err = err
	}
}

func (m *Model) AddClient(addr string) {
	ip, port, err := ToHostPort(addr)
	if err != nil {
		m.err = err
		return
	}
	m.AppCfg.ManualAddresses = append(m.AppCfg.ManualAddresses, addr)
	_ = SaveAppConfig(m.AppCfg)

	m.Relay.AddClient(ip, port)
}

func (m *Model) RemoveClients(selected map[string]struct{}) {
	manualSet := make(map[string]struct{}, len(m.AppCfg.ManualAddresses))
	for _, addr := range m.AppCfg.ManualAddresses {
		manualSet[addr] = struct{}{}
	}
	for addr := range selected {
		if _, ok := manualSet[addr]; ok {
			delete(manualSet, addr)
		}
	}
	m.AppCfg.ManualAddresses = make([]string, 0, len(manualSet))
	for addr := range manualSet {
		m.AppCfg.ManualAddresses = append(m.AppCfg.ManualAddresses, addr)
	}
	_ = SaveAppConfig(m.AppCfg)

	m.Relay.RemoveClients(selected)
}

func ToHostPort(addr string, defaultPort ...int) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
		portStr = ""
	}
	if ip := net.ParseIP(host); ip == nil {
		return "", 0, fmt.Errorf("invalid ip address %s", host)
	}
	var port int
	if portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("invalid port: %w", err)
		}
	} else {
		if len(defaultPort) == 0 {
			return "", 0, fmt.Errorf("missing port in address %s", addr)
		}
		port = defaultPort[0]
	}
	if port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("invalid port %d", port)
	}
	return host, port, nil
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

func (m Model) sortedClients(clients map[string]*relay.Client) ([]*relay.Client, int) {
	manualSet := make(map[string]struct{}, len(m.AppCfg.ManualAddresses))
	for _, addr := range m.AppCfg.ManualAddresses {
		manualSet[addr] = struct{}{}
	}

	auto := make([]*relay.Client, 0, len(clients))
	manual := make([]*relay.Client, 0, len(clients))
	for _, k := range sortedClientKeys(clients) {
		if _, ok := manualSet[k]; ok {
			manual = append(manual, clients[k])
		} else {
			auto = append(auto, clients[k])
		}
	}

	autoCount := len(auto)
	all := append(auto, manual...)
	return all, autoCount
}

func sortedClientKeys(clients map[string]*relay.Client) []string {
	addrs := make([]string, 0, len(clients))
	for k := range clients {
		addrs = append(addrs, k)
	}
	sort.Strings(addrs)
	return addrs
}
