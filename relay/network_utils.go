package relay

import (
	"net"
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
