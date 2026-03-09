package network

import (
	"fmt"
	"net"
	"strconv"
)

func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 |
		uint32(ip[1])<<16 |
		uint32(ip[2])<<8 |
		uint32(ip[3])
}

func uint32ToIP(n uint32) net.IP {
	return net.IPv4(
		byte(n>>24),
		byte((n>>16)&0xff),
		byte((n>>8)&0xff),
		byte(n&0xff),
	)
}

// GetLocalSubnet returns the subnet of the first non-loopback, up, IPv4 network interface.
func GetLocalSubnet() (*net.IPNet, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("cannot list network interfaces: %w", err)
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil {
				continue
			}
			if ip4[0] == 127 {
				continue
			}
			return &net.IPNet{
				IP:   ip4.Mask(ipnet.Mask),
				Mask: ipnet.Mask,
			}, nil
		}
	}
	return nil, fmt.Errorf("no suitable network interface found")
}

func getSubnetUsableRange(ipnet *net.IPNet) (uint32, uint32) {
	mask := ipnet.Mask
	bcast := make(net.IP, len(ipnet.IP))
	for i := 0; i < len(ipnet.IP); i++ {
		bcast[i] = ipnet.IP[i] | ^mask[i]
	}
	min := ipToUint32(ipnet.IP) + 1
	max := ipToUint32(bcast) - 1
	return min, max
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
