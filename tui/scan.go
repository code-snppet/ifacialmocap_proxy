package tui

import (
	"context"
	"fmt"
	"net"
	"time"

	"codesnppet.dev/ifmproxy/network"
	tea "github.com/charmbracelet/bubbletea"
)

type ScanResultMsg struct {
	Addr *net.UDPAddr
	Err  error
}

type AutoConnectTickMsg struct{}

func scheduleAutoConnectCheck() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return AutoConnectTickMsg{}
	})
}

func (m *Model) Scan(subnet string) tea.Cmd {
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		m.err = err
		m.logger.Error(fmt.Sprintf("Failed to parse subnet %s: %s", subnet, err))
		return nil
	}

	return m.scanSubnet(ipnet)
}

func (m *Model) scanSubnet(ipnet *net.IPNet) tea.Cmd {
	m.cancelActiveScan()
	m.Scanning = true
	listenAddr := m.Relay.ListenAddr()
	m.Relay.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	m.scanCancel = cancel

	return func() tea.Msg {
		laddr, err := net.ResolveUDPAddr(network.RELAY_NETWORK, listenAddr)
		if err != nil {
			return ScanResultMsg{Err: fmt.Errorf("resolve listen addr: %w", err)}
		}
		finder := network.NewIFMDeviceFinder(ipnet, laddr, m.logger)
		addr, err := finder.FindIFM(ctx)
		return ScanResultMsg{Addr: addr, Err: err}
	}
}

func (m *Model) cancelActiveScan() {
	if m.scanCancel != nil {
		m.scanCancel()
		m.scanCancel = nil
	}
}

func (m *Model) CancelScan() {
	m.autoConnecting = false
	m.cancelActiveScan()
}

func (m *Model) handleScanResult(msg ScanResultMsg) tea.Cmd {
	m.Scanning = false
	m.scanCancel = nil

	if msg.Err != nil && msg.Err != context.Canceled {
		m.logger.Error(fmt.Sprintf("Scan error: %s", msg.Err))
	}

	if msg.Addr != nil {
		addr := msg.Addr.String()
		m.AppCfg.Remote = addr
		_ = SaveAppConfig(m.AppCfg)
		m.logger.Info(fmt.Sprintf("Scan found device at %s", addr))
		// Set remote before Start so the relay uses the new address
		if err := m.Relay.SetRemote(msg.Addr.IP.String(), msg.Addr.Port); err != nil {
			m.logger.Error(fmt.Sprintf("Failed to set remote: %s", err))
		}
		m.autoConnecting = false
	}

	if err := m.Relay.Start(); err != nil {
		m.logger.Error(fmt.Sprintf("Failed to restart relay: %s", err))
	}

	cmds := []tea.Cmd{waitRelaySignal(m.Relay)}
	if m.autoConnecting && msg.Addr == nil {
		m.logger.Debug("Auto-connect: no address found, rescheduling")
		cmds = append(cmds, scheduleAutoConnectCheck())
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleAutoConnectTick() tea.Cmd {
	if !m.autoConnecting {
		return nil
	}

	if m.Relay.IsUpstreamAlive() {
		m.autoConnecting = false
		m.logger.Info("Auto-connect: upstream is alive")
		return nil
	}

	if m.Scanning {
		return scheduleAutoConnectCheck()
	}

	subnet, err := network.GetLocalSubnet()
	if err != nil {
		m.logger.Error(fmt.Sprintf("Auto-connect: %s", err))
		return scheduleAutoConnectCheck()
	}

	m.logger.Info(fmt.Sprintf("Auto-connect: scanning %s", subnet))
	return m.scanSubnet(subnet)
}
