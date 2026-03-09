package relay

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"codesnppet.dev/ifmproxy/logger"
)

type IFMDeviceFinder struct {
	ipnet      *net.IPNet
	listenAddr *net.UDPAddr
	logger     *logger.Logger
	localIPs   map[string]struct{}
	foundChan  chan *net.UDPAddr
	stopOnce   sync.Once
	stopCh     chan struct{}
	conn       *net.UDPConn
	errCh      chan error
}

func NewIFMDeviceFinder(ipnet *net.IPNet, listenAddr *net.UDPAddr, logger *logger.Logger) *IFMDeviceFinder {
	return &IFMDeviceFinder{
		ipnet:      ipnet,
		listenAddr: listenAddr,
		logger:     logger,
		localIPs:   getLocalIPs(),
		foundChan:  make(chan *net.UDPAddr, 1),
		stopCh:     make(chan struct{}),
		errCh:      make(chan error, 1),
	}
}

// getLocalIPs returns the set of all unicast IPs assigned to this machine.
func getLocalIPs() map[string]struct{} {
	out := make(map[string]struct{})
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return out
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		out[ipnet.IP.String()] = struct{}{}
	}
	return out
}

func (d *IFMDeviceFinder) isLocalIP(ip net.IP) bool {
	_, local := d.localIPs[ip.String()]
	return local
}

func (d *IFMDeviceFinder) listen() error {
	conn, err := net.ListenUDP(RELAY_NETWORK, d.listenAddr)
	if err != nil {
		return err
	}
	d.conn = conn
	return nil
}

func (d *IFMDeviceFinder) readLoop() {
	buf := make([]byte, 8192)
	for {
		if d.isStopping() {
			return
		}
		d.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, from, err := d.conn.ReadFromUDP(buf)
		if err != nil {
			if d.isStopping() {
				return
			}
			// Deadline exceeded is expected — just retry.
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			d.errCh <- err
			return
		}
		if n > 0 && !d.isLocalIP(from.IP) {
			d.foundChan <- from
		}
	}
}

func (d *IFMDeviceFinder) Stop() {
	d.stopOnce.Do(func() {
		close(d.stopCh)
		if d.conn != nil {
			d.conn.Close()
		}
	})
}

func (d *IFMDeviceFinder) isStopping() bool {
	select {
	case <-d.stopCh:
		return true
	default:
		return false
	}
}

func (d *IFMDeviceFinder) FindIFM(ctx context.Context) (*net.UDPAddr, error) {
	iterator := NewSubnetIterator(d.ipnet)

	err := d.listen()
	if err != nil {
		return nil, err
	}
	go d.readLoop()
	defer d.Stop()

	for iterator.HasNext() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case addr := <-d.foundChan:
			d.debug(fmt.Sprintf("Found IFM device %s", addr.String()))
			return addr, nil
		case err := <-d.errCh:
			return nil, fmt.Errorf("read error during scan: %w", err)
		default:
		}

		ip := iterator.Next()
		if d.isLocalIP(ip) {
			continue
		}
		addr, err := net.ResolveUDPAddr(RELAY_NETWORK, fmt.Sprintf("%s:%d", ip.String(), IFM_PORT))
		if err != nil {
			d.debug(fmt.Sprintf("Error resolving UDP address %s: %s", ip.String(), err))
			continue
		}
		_, err = d.conn.WriteToUDP(BYTES_IFM_CONNECTION_COMMAND, addr)
		if err != nil {
			d.debug(fmt.Sprintf("Error writing to UDP address %s: %s", ip.String(), err))
			continue
		}
		d.debug(fmt.Sprintf("Sent connection command to %s", addr.String()))
	}

	d.debug("All probes sent, waiting for IFM device response...")
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case addr := <-d.foundChan:
		d.debug(fmt.Sprintf("Found IFM device %s", addr.String()))
		return addr, nil
	case err := <-d.errCh:
		return nil, fmt.Errorf("read error while waiting: %w", err)
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("no IFM device found")
	}
}

func (d *IFMDeviceFinder) debug(message string) {
	if d.logger == nil {
		return
	}
	d.logger.Debug(message)
}
