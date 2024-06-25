package client

import (
	"context"
	"errors"
	"fmt"
	"github.com/BaiMeow/aict/proto"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/time/rate"
	"log"
	"math"
	"net"
	"sync/atomic"
	"time"
)

const (
	bufferSize     = 1700
	bufferQueueLen = 1024
	boostPeriod    = 500 * time.Millisecond
	RTT            = 10 * time.Millisecond
)

type AictConn struct {
	conn         net.PacketConn
	raddr        *net.IPAddr
	identify     int
	readBuffer   chan []byte
	writeBuffer  chan []byte
	sequence     atomic.Uint32
	peerSequence atomic.Uint32
	readCounter  atomic.Uint32
	cancel       context.CancelFunc
	ctx          context.Context

	peerQueueSize    int
	sentSequenceN    int
	minSentSequenceN int
	maxSentSequenceN int

	sendLimiter *rate.Limiter

	sequenceTimer *time.Timer
}

func newAict(conn net.PacketConn, raddr *net.IPAddr, cfg *Config) *AictConn {
	ctx, cancel := context.WithCancel(context.Background())
	c := &AictConn{
		conn:             conn,
		raddr:            raddr,
		identify:         cfg.Identify,
		cancel:           cancel,
		readBuffer:       make(chan []byte, bufferQueueLen),
		writeBuffer:      make(chan []byte, bufferQueueLen),
		ctx:              ctx,
		sentSequenceN:    cfg.minAirSeqCount,
		minSentSequenceN: cfg.minAirSeqCount,
		maxSentSequenceN: cfg.maxAirSeqCount,
		sequenceTimer:    time.NewTimer(boostPeriod / time.Duration(cfg.minAirSeqCount)),
		sendLimiter:      rate.NewLimiter(rate.Every(RTT), 1),
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
			//cost := max(uint32(c.sentSequenceN)-(c.sequence.Load()-c.peerSequence.Load()), 1)
			// ensure fold [0,1]
			//calc := float64(cost) / 0.6
			calc := float64(count)/0.6*0.5 + float64(c.sentSequenceN)*0.5
			calc = min(max(calc, float64(c.minSentSequenceN)), float64(c.maxSentSequenceN))
			log.Printf("icmp: air seq count %d", int(calc))
			c.sentSequenceN = int(calc)
			c.sequenceTimer.Stop()
			// notify write routine to reset timer
			c.sequenceTimer.Reset(1)
		}
	}
}

func (c *AictConn) cancelSeqOnce() {
	c.sequenceTimer.Stop()
	c.sequenceTimer.Reset(boostPeriod / time.Duration(c.sentSequenceN))
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

		seq := uint16(uint32(echo.Seq) & 0xffff)

		c.readCounter.Add(1)
	UpdatePeerSeq:
		old := c.peerSequence.Load()
		if seq-uint16(old) < math.MaxUint16/2 {
			if !c.peerSequence.CompareAndSwap(old, uint32(seq)) {
				goto UpdatePeerSeq
			}
		}

		msg := &proto.Layer{}
		if err := msg.Unmarshal(echo.Data); err != nil {
			fmt.Println("unmarshal error: ", err)
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
			c.sequenceTimer.Stop()
			return nil
		case aictLayer.Payload = <-c.writeBuffer:
			c.cancelSeqOnce()
		case <-c.sequenceTimer.C:
			c.sequenceTimer.Reset(boostPeriod / time.Duration(c.sentSequenceN))
			aictLayer.Flags = proto.FlagKeepalive
		}

		// rate limit
		err := c.sendLimiter.Wait(c.ctx)
		if err != nil {
			return fmt.Errorf("rate limit: %v", err)
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
				Seq:  int(uint16(c.sequence.Add(1))),
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
