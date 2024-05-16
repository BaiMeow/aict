package server

import (
	"fmt"
	"golang.org/x/net/icmp"
	"net"
)

type Config struct {
	SeqQueueSize int
}

func Listen(laddr *net.IPAddr, raddr *net.IPAddr, cfg *Config) (*AictConn, error) {
	conn, err := icmp.ListenPacket("ip4:icmp", laddr.String())
	if err != nil {
		return nil, fmt.Errorf("icmp: listen: %v", err)
	}

	if cfg.SeqQueueSize == 0 {
		cfg.SeqQueueSize = 16
	}

	return newAict(conn, raddr, cfg), nil
}
