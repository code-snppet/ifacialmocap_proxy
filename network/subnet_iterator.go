package network

import "net"

type SubnetIterator struct {
	min uint32
	max uint32
	cur uint32
}

func NewSubnetIterator(ipnet *net.IPNet) *SubnetIterator {
	min, max := getSubnetUsableRange(ipnet)
	return &SubnetIterator{
		min: min,
		max: max,
		cur: min,
	}
}

func (s *SubnetIterator) Next() net.IP {
	if s.cur >= s.max {
		return nil
	}
	s.cur++
	return uint32ToIP(s.cur)
}

func (s *SubnetIterator) HasNext() bool {
	return s.cur < s.max
}

func (s *SubnetIterator) Reset() {
	s.cur = s.min
}

func (s *SubnetIterator) Value() net.IP {
	return uint32ToIP(s.cur)
}

func (s *SubnetIterator) Min() net.IP {
	return uint32ToIP(s.min)
}

func (s *SubnetIterator) Max() net.IP {
	return uint32ToIP(s.max)
}
