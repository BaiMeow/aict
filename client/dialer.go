package client

import (
	"fmt"
	"golang.org/x/net/icmp"
	"math"
	"math/rand/v2"
	"net"
)

type Config struct {
	Identify       int
	minAirSeqCount int
	maxAirSeqCount int
}

func Dial(laddr *net.IPAddr, raddr *net.IPAddr, cfg *Config) (*AictConn, error) {
	conn, err := icmp.ListenPacket("ip4:icmp", laddr.String())
	if err != nil {
		return nil, fmt.Errorf("icmp: listen: %v", err)
	}

	if cfg.Identify == 0 {
		cfg.Identify = rand.IntN(math.MaxUint16)
	}
	if cfg.minAirSeqCount == 0 {
		cfg.minAirSeqCount = 1
	}
	if cfg.maxAirSeqCount == 0 {
		cfg.maxAirSeqCount = 32
	}

	return newAict(conn, raddr, cfg), nil
}
