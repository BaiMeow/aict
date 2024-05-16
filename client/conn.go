package client

import (
	"context"
	"errors"
	"fmt"
	"github.com/BaiMeow/aict/proto"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"log"
	"math"
	"net"
	"sync/atomic"
	"time"
)

const (
	bufferSize     = 1500
	bufferQueueLen = 1024
	boostPeriod    = 500 * time.Millisecond
)

type AictConn struct {
	conn        net.PacketConn
	raddr       *net.IPAddr
	identify    int
	readBuffer  chan []byte
	writeBuffer chan []byte
	// sequence is uint16 but only uint32 has atomic operation
	sequence     uint32
	peerSequence uint32
	cancel       context.CancelFunc
	ctx          context.Context

	airSequenceN    int
	minAirSequenceN int
	maxAirSequenceN int

	sequenceTicker *time.Ticker
	readCounter    atomic.Uint32
}

func newAict(conn net.PacketConn, raddr *net.IPAddr, cfg *Config) *AictConn {
	ctx, cancel := context.WithCancel(context.Background())
	c := &AictConn{
		conn:            conn,
		raddr:           raddr,
		identify:        cfg.Identify,
		cancel:          cancel,
		readBuffer:      make(chan []byte, bufferQueueLen),
		writeBuffer:     make(chan []byte, bufferQueueLen),
		ctx:             ctx,
		airSequenceN:    cfg.minAirSeqCount,
		minAirSequenceN: cfg.minAirSeqCount,
		maxAirSequenceN: cfg.maxAirSeqCount,
		sequenceTicker:  time.NewTicker(boostPeriod / time.Duration(cfg.minAirSeqCount)),
	}
	go func() {
		err := c.readRoutine()
		if err == nil {
			return
		}
		log.Printf("read routine: %v", err)
		err = c.Close()
		if err == nil {
			return
		}
		log.Printf("close: %v", err)
	}()
	go func() {
		err := c.writeRoutine()
		if err == nil {
			return
		}
		log.Printf("write routine: %v", err)
		err = c.Close()
		if err == nil {
			return
		}
		log.Printf("close: %v", err)
	}()
	go c.booster()
	return c
}

func (c *AictConn) Close() error {
	c.cancel()
	return c.conn.Close()
}

func (c *AictConn) booster() {
	tBoost := time.NewTicker(boostPeriod)
	for {
		select {
		case <-c.ctx.Done():
			tBoost.Stop()
			return
		case <-tBoost.C:
			count := c.readCounter.Swap(0)
			// ensure fold [0,1]
			fold := min(float64(count)/float64(c.airSequenceN), 1)
			calc := math.Round((math.Pow(fold, 1.6)-0.5)*float64(c.airSequenceN) + float64(c.airSequenceN))
			calc = min(max(calc, float64(c.minAirSequenceN)), float64(c.maxAirSequenceN))
			// log.Printf("icmp: air seq count %d", int(calc))
			c.airSequenceN = int(calc)
			c.cancelSeqOnce()
		}
	}
}

func (c *AictConn) cancelSeqOnce() {
	c.sequenceTicker.Reset(boostPeriod / time.Duration(c.airSequenceN))
}

func (c *AictConn) readRoutine() error {
	for {
		select {
		case <-c.ctx.Done():
			return nil
		default:
		}

		buf := make([]byte, bufferSize)
		err := c.conn.SetReadDeadline(time.Now().Add(time.Second * 30))
		if err != nil {
			// exit
			if err := c.Close(); err != nil {
				log.Printf("close: %v", err)
			}
			return fmt.Errorf("set read readline: %v", err)
		}
		n, addr, err := c.conn.ReadFrom(buf)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			// exit
			if err := c.Close(); err != nil {
				log.Printf("close: %v", err)
			}
			return fmt.Errorf("read packet: %v", err)
		}

		ipAddr, ok := addr.(*net.IPAddr)
		if !ok {
			return fmt.Errorf("under conn addr type not ip addr")
		}
		if !ipAddr.IP.Equal(c.raddr.IP) {
			continue
		}

		m, err := icmp.ParseMessage(1, buf[:n])
		if err != nil {
			log.Printf("aict: parse: %v", err)
			continue
		}
		echo, ok := m.Body.(*icmp.Echo)
		if !ok || m.Type != ipv4.ICMPTypeEchoReply {
			continue
		}

		c.readCounter.Add(1)

		msg := &proto.Layer{}
		if err := msg.Unmarshal(echo.Data); err != nil {
			// skip
			continue
		}

		if msg.Flags&proto.FlagKeepalive > 0 {
			continue
		}
		c.readBuffer <- msg.Payload
	}
}

func (c *AictConn) writeRoutine() error {
	for {
		aictLayer := proto.Layer{
			Flags: 0,
		}
		select {
		case <-c.ctx.Done():
			c.sequenceTicker.Stop()
			return nil
		case aictLayer.Payload = <-c.writeBuffer:
			c.cancelSeqOnce()
		case <-c.sequenceTicker.C:
			aictLayer.Flags = proto.FlagKeepalive
		}

		data, err := aictLayer.Marshal()
		if err != nil {
			log.Printf("marshal aict layer: %v", err)
			continue
		}
		msg := icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{
				ID:   c.identify,
				Seq:  int(uint16(atomic.AddUint32(&c.sequence, 1))),
				Data: data,
			},
		}
		raw, err := msg.Marshal(nil)
		if err != nil {
			log.Printf("marshal icmp message: %v", err)
		}
		_, err = c.conn.WriteTo(raw, c.raddr)
		if err != nil {
			return fmt.Errorf("write to conn: %v", err)
		}
	}
}

func (c *AictConn) WritePacket(data []byte) error {
	select {
	case <-c.ctx.Done():
		return errors.New("connection closed")
	default:
	}
	c.writeBuffer <- data
	return nil
}

func (c *AictConn) ReadPacket() ([]byte, error) {
	select {
	case <-c.ctx.Done():
		return nil, errors.New("connection closed")
	case buf := <-c.readBuffer:
		if buf == nil {
			return nil, errors.New("connection closed")
		}
		return buf, nil
	}
}
