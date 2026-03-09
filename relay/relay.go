package relay

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	IFM_PORT                   = 49983
	IFM_CONNECTION_COMMAND     = "iFacialMocap_sahuasouryya9218sauhuiayeta91555dy3719"
	IFM_V2_STRING              = "|sendDataVersion=v2"
	IFM_TCP_CONNECTION_COMMAND = "iFacialMocap_UDPTCP_sahuasouryya9218sauhuiayeta91555dy3719"
	IFM_TCP_STOP_COMMAND       = "iFacialMocap_UDPTCPSTOP_sahuasouryya9218sauhuiayeta91555dy3719"
)

var (
	BYTES_IFM_CONNECTION_COMMAND     = []byte(IFM_CONNECTION_COMMAND)
	BYTES_IFM_V2_STRING              = []byte(IFM_V2_STRING)
	BYTES_IFM_TCP_CONNECTION_COMMAND = []byte(IFM_TCP_CONNECTION_COMMAND)
	BYTES_IFM_TCP_STOP_COMMAND       = []byte(IFM_TCP_STOP_COMMAND)
)

type Status int

const (
	STATUS_STOPPED Status = iota
	STATUS_WAITING
	STATUS_GOOD
)

type Cfg struct {
	Listen string
	Remote string
}

type Stats struct {
	Received     int
	Sent         int
	LastPacket   []byte
	LastPacketAt time.Time
}

type Node struct {
	Status Status
	Addr   *net.UDPAddr
	Stats  Stats
}

type (
	Upstream Node
	Client   Node
)

type RelaySnapshot struct {
	Status     Status
	ListenAddr string
	RemoteAddr string
	Upstream   *Upstream
	Clients    map[string]*Client
	LastErr    error
}

type Relay struct {
	mu        sync.RWMutex
	cfg       Cfg
	conn      *net.UDPConn
	upstream  *Upstream
	clients   map[string]*Client
	started   bool
	status    Status
	lastErr   error
	notify    chan struct{}
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

const RELAY_NETWORK = "udp4"

func NewRelay(cfg Cfg) *Relay {
	return &Relay{
		cfg:       cfg,
		clients:   make(map[string]*Client),
		notify:    make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

func NewUpstream(addr *net.UDPAddr) *Upstream {
	return &Upstream{
		Status: STATUS_STOPPED,
		Addr:   addr,
	}
}

func NewClient(addr *net.UDPAddr) *Client {
	return &Client{
		Status: STATUS_GOOD,
		Addr:   addr,
	}
}

func (r *Relay) NotifyChan() <-chan struct{} {
	return r.notify
}

func (r *Relay) Snapshot() RelaySnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snap := RelaySnapshot{
		Status:     r.status,
		ListenAddr: r.cfg.Listen,
		RemoteAddr: r.cfg.Remote,
		Clients:    make(map[string]*Client, len(r.clients)),
		LastErr:    r.lastErr,
	}
	if r.upstream != nil {
		u := *r.upstream
		snap.Upstream = &u
	}
	for k, v := range r.clients {
		client := *v
		snap.Clients[k] = &client
	}
	return snap
}

func (r *Relay) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return nil
	}

	laddr, err := net.ResolveUDPAddr(RELAY_NETWORK, r.cfg.Listen)
	if err != nil {
		return fmt.Errorf("invalid listen addr: %w", err)
	}

	conn, err := net.ListenUDP(RELAY_NETWORK, laddr)
	if err != nil {
		return fmt.Errorf("cannot open udp port: %w", err)
	}

	r.conn = conn
	r.started = true
	r.status = STATUS_WAITING
	r.stopCh = make(chan struct{})
	r.stoppedCh = make(chan struct{})
	go r.readLoop()
	go r.heartbeatLoop()

	if r.cfg.Remote != "" {
		parts := strings.Split(r.cfg.Remote, ":")
		ip := parts[0]
		port := IFM_PORT
		if len(parts) > 1 {
			port, err = strconv.Atoi(parts[1])
			if err != nil {
				return fmt.Errorf("invalid remote port: %w", err)
			}
		}
		return r.setRemoteLocked(ip, port)
	}

	return nil
}

func (r *Relay) sendHandshake() error {
	if r.upstream == nil || r.conn == nil {
		return nil
	}
	_, err := r.conn.WriteToUDP(BYTES_IFM_CONNECTION_COMMAND, r.upstream.Addr)
	if err != nil {
		return fmt.Errorf("attempt to send handshake: %w", err)
	}
	r.upstream.Status = STATUS_WAITING
	return nil
}

func (r *Relay) Stop() {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return
	}
	r.status = STATUS_STOPPED
	if r.upstream != nil {
		r.upstream.Status = STATUS_STOPPED
	}
	r.started = false
	close(r.stopCh)
	conn := r.conn
	r.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	<-r.stoppedCh
}

func (r *Relay) setRemoteLocked(ip string, port int) error {
	if !r.started {
		return fmt.Errorf("relay not started")
	}

	if ip == "" {
		r.upstream = nil
		r.cfg.Remote = ""
		return nil
	}

	addr, err := net.ResolveUDPAddr(RELAY_NETWORK, fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return fmt.Errorf("invalid remote address: %w", err)
	}
	r.upstream = NewUpstream(addr)
	r.cfg.Remote = addr.String()
	return r.sendHandshake()
}

func (r *Relay) SetListen(ip string, port int) error {
	r.mu.Lock()
	r.cfg.Listen = fmt.Sprintf("%s:%d", ip, port)
	r.mu.Unlock()
	if r.started {
		r.Stop()
	}
	return r.Start()
}

func (r *Relay) SetRemote(ip string, port int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.setRemoteLocked(ip, port)
}

func (r *Relay) AddClient(ip string, port int) {
	go func() {
		addr, err := net.ResolveUDPAddr(RELAY_NETWORK, fmt.Sprintf("%s:%d", ip, port))
		if err != nil {
			r.setErr(fmt.Errorf("invalid client address: %w", err))
			return
		}
		client := NewClient(addr)
		r.mu.Lock()
		r.clients[addr.String()] = client
		r.mu.Unlock()
		r.signal()
	}()
}

func (r *Relay) RemoveClients(clients map[string]struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k := range clients {
		_, exists := r.clients[k]
		if exists {
			delete(r.clients, k)
		}
	}
	r.signal()
}

func (r *Relay) heartbeatLoop() {
	for {
		if r.isStopping() {
			return
		}
		r.sendHandshake()
		time.Sleep(10 * time.Second)
	}
}

func (r *Relay) readLoop() {
	defer close(r.stoppedCh)

	buf := make([]byte, 8192)

	for {
		if r.isStopping() {
			return
		}

		n, from, err := r.conn.ReadFromUDP(buf)
		if err != nil {
			if r.isStopping() {
				return
			}
			r.setErr(err)
			continue
		}

		if n <= 0 {
			continue
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		if r.isHandshake(data) {
			r.handleHandshake(from, data)
		} else {
			r.handleDataPacket(from, data)
		}
	}
}

func (r *Relay) isHandshake(data []byte) bool {
	return bytes.Equal(data, BYTES_IFM_CONNECTION_COMMAND)
}

func (r *Relay) handleHandshake(from *net.UDPAddr, data []byte) {
	r.mu.Lock()
	key := from.String()
	client, exists := r.clients[key]
	if !exists {
		client = NewClient(from)
		r.clients[key] = client
	}
	client.Stats.LastPacket = data
	client.Stats.LastPacketAt = time.Now()
	client.Stats.Received++
	r.status = STATUS_GOOD
	r.lastErr = nil
	r.mu.Unlock()

	r.signal()
}

func (r *Relay) handleDataPacket(from *net.UDPAddr, data []byte) {
	r.mu.RLock()
	if r.upstream == nil {
		r.mu.RUnlock()
		return
	}
	if !from.IP.Equal(r.upstream.Addr.IP) {
		r.mu.RUnlock()
		return
	}

	clientsCopy := make(map[string]*Client, len(r.clients))
	for k, v := range r.clients {
		clientsCopy[k] = v
	}
	r.mu.RUnlock()

	relayedTo := make([]*Client, 0, len(clientsCopy))
	for key, client := range clientsCopy {
		if addrEquals(from, client.Addr) {
			continue
		}
		if _, werr := r.conn.WriteToUDP(data, client.Addr); werr != nil {
			r.setErr(fmt.Errorf("write to %s: %w", key, werr))
			continue
		}
		relayedTo = append(relayedTo, client)
	}

	r.mu.Lock()
	r.status = STATUS_GOOD
	r.lastErr = nil
	r.upstream.Status = STATUS_GOOD
	r.upstream.Stats.Received++
	r.upstream.Stats.LastPacket = data
	r.upstream.Stats.LastPacketAt = time.Now()
	for _, client := range relayedTo {
		client.Stats.Sent++
		client.Status = STATUS_GOOD
	}
	r.mu.Unlock()

	r.signal()
}

func (r *Relay) setErr(err error) {
	r.mu.Lock()
	r.lastErr = err
	r.mu.Unlock()
	r.signal()
}

func (r *Relay) signal() {
	select {
	case r.notify <- struct{}{}:
	default:
	}
}

func (r *Relay) isStopping() bool {
	select {
	case <-r.stopCh:
		return true
	default:
		return false
	}
}

func addrEquals(a, b *net.UDPAddr) bool {
	return a.Port == b.Port && a.IP.Equal(b.IP)
}
