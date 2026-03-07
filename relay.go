package main

import (
	"bytes"
	"fmt"
	"net"
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
	listen string
	remote string
}

type Stats struct {
	received     int
	sent         int
	lastPacket   []byte
	lastPacketAt time.Time
}

type Node struct {
	status Status
	addr   *net.UDPAddr
	stats  Stats
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
		status: STATUS_STOPPED,
		addr:   addr,
	}
}

func NewClient(addr *net.UDPAddr) *Client {
	return &Client{
		status: STATUS_GOOD,
		addr:   addr,
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
		ListenAddr: r.cfg.listen,
		RemoteAddr: r.cfg.remote,
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

	laddr, err := net.ResolveUDPAddr(RELAY_NETWORK, r.cfg.listen)
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
	go r.readLoop()

	if r.cfg.remote != "" {
		return r.setRemoteLocked(r.cfg.remote)
	}

	return nil
}

func (r *Relay) sendHandshake() error {
	if r.upstream == nil || r.conn == nil {
		return nil
	}
	_, err := r.conn.WriteToUDP(BYTES_IFM_CONNECTION_COMMAND, r.upstream.addr)
	if err != nil {
		return fmt.Errorf("attempt to send handshake: %w", err)
	}
	r.upstream.status = STATUS_WAITING
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
		r.upstream.status = STATUS_STOPPED
	}
	close(r.stopCh)
	conn := r.conn
	r.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	<-r.stoppedCh
}

func (r *Relay) setRemoteLocked(ip string) error {
	if !r.started {
		return fmt.Errorf("relay not started")
	}

	if ip == "" {
		r.upstream = nil
		r.cfg.remote = ""
		return nil
	}

	addr, err := net.ResolveUDPAddr(RELAY_NETWORK, fmt.Sprintf("%s:%d", ip, IFM_PORT))
	if err != nil {
		return fmt.Errorf("invalid remote address: %w", err)
	}
	r.upstream = NewUpstream(addr)
	r.cfg.remote = addr.String()
	return r.sendHandshake()
}

func (r *Relay) SetRemote(ip string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.setRemoteLocked(ip)
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
	client.stats.lastPacket = data
	client.stats.lastPacketAt = time.Now()
	client.stats.received++
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
	if r.upstream != nil && !from.IP.Equal(r.upstream.addr.IP) {
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
		if addrEquals(from, client.addr) {
			continue
		}
		if _, werr := r.conn.WriteToUDP(data, client.addr); werr != nil {
			r.setErr(fmt.Errorf("write to %s: %w", key, werr))
			continue
		}
		relayedTo = append(relayedTo, client)
	}

	r.mu.Lock()
	r.status = STATUS_GOOD
	r.lastErr = nil
	r.upstream.status = STATUS_GOOD
	r.upstream.stats.received++
	r.upstream.stats.lastPacket = data
	r.upstream.stats.lastPacketAt = time.Now()
	for _, client := range relayedTo {
		client.stats.sent++
		client.status = STATUS_GOOD
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
