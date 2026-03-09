package tui

import (
	"fmt"
	"sort"

	"codesnppet.dev/ifmproxy/network"
)

func (m *Model) ConnectTo(addr string) {
	ip, port, err := network.ToHostPort(addr, network.IFM_PORT)
	if err != nil {
		m.err = err
		return
	}
	m.CancelScan()
	m.AppCfg.Remote = addr
	_ = SaveAppConfig(m.AppCfg)
	m.logger.Info(fmt.Sprintf("Connecting to %s", addr))
	if err := m.Relay.SetRemote(ip, port); err != nil {
		m.err = err
		m.logger.Error(fmt.Sprintf("Failed to connect to %s: %s", addr, err))
	}
}

func (m *Model) ListenTo(addr string) {
	ip, port, err := network.ToHostPort(addr)
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
	ip, port, err := network.ToHostPort(addr)
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

func (m Model) sortedClients(clients map[string]*network.Client) ([]*network.Client, int) {
	manualSet := make(map[string]struct{}, len(m.AppCfg.ManualAddresses))
	for _, addr := range m.AppCfg.ManualAddresses {
		manualSet[addr] = struct{}{}
	}

	auto := make([]*network.Client, 0, len(clients))
	manual := make([]*network.Client, 0, len(clients))
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

func sortedClientKeys(clients map[string]*network.Client) []string {
	addrs := make([]string, 0, len(clients))
	for k := range clients {
		addrs = append(addrs, k)
	}
	sort.Strings(addrs)
	return addrs
}
