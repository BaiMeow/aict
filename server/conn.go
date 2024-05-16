package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/BaiMeow/aict/ds"
	"github.com/BaiMeow/aict/proto"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"log"
	"net"
	"time"
)

const bufferSize = 1500

type AictConn struct {
	conn        net.PacketConn
	readBuffer  chan []byte
	writeBuffer chan []byte
	initialized chan struct{}

	cancel context.CancelFunc
	ctx    context.Context

	raddr             *net.IPAddr
	identify          uint16
	sequenceQueue     *ds.RotatedQueue[uint16]
	sequenceQueueSize int
}

func newAict(c net.PacketConn, raddr *net.IPAddr, cfg *Config) *AictConn {
	ctx, cancel := context.WithCancel(context.Background())
	aict := &AictConn{
		conn:              c,
		readBuffer:        make(chan []byte, 1024),
		writeBuffer:       make(chan []byte, 1024),
		initialized:       make(chan struct{}),
		ctx:               ctx,
		cancel:            cancel,
		raddr:             raddr,
		sequenceQueueSize: cfg.SeqQueueSize,
		sequenceQueue:     ds.NewRotatedQueue[uint16](cfg.SeqQueueSize),
	}
	go func() {
		err := aict.readRoutine()
		if err != nil {
			log.Printf("exit read loop: %v", err)
		}
	}()
	go func() {
		err := aict.writeRoutine()
		if err != nil {
			log.Printf("exit write loop: %v", err)
		}
	}()
	return aict
}

func (c *AictConn) Close() error {
	c.cancel()
	return c.conn.Close()
}

func (c *AictConn) readRoutine() error {
	buf := make([]byte, bufferSize)
	for {
		// check context
		select {
		case <-c.ctx.Done():
			return nil
		default:
		}

		err := c.conn.SetReadDeadline(time.Now().Add(time.Second * 30))
		if err != nil {
			return fmt.Errorf("icmp: set read deadline: %v", err)
		}
		n, addr, err := c.conn.ReadFrom(buf)
		if err != nil {
			// if not init
			if err, ok := err.(net.Error); ok && c.identify == 0 && err.Timeout() {
				continue
			}
			return fmt.Errorf("icmp: read from: %v", err)
		}

		m, err := icmp.ParseMessage(1, buf[:n])
		if err != nil {
			log.Printf("icmp: parse message: %v", err)
			continue
		}

		echo, ok := m.Body.(*icmp.Echo)
		if !ok || m.Type != ipv4.ICMPTypeEcho {
			continue
		}

		msg := proto.Layer{}
		if err := msg.Unmarshal(echo.Data); err != nil {
			continue
		}

		ipaddr, ok := addr.(*net.IPAddr)
		if !ok {
			return errors.New("PacketConn not return IPAddr")
		}
		// do init
		if c.identify == 0 && (c.raddr.IP.Equal(net.IP{0, 0, 0, 0}) || c.raddr.IP.Equal(ipaddr.IP)) {
			c.identify = uint16(echo.ID)
			c.sequenceQueue = ds.NewRotatedQueue[uint16](c.sequenceQueueSize)
			c.raddr = ipaddr
			close(c.initialized)
			log.Println("accept connection from " + c.raddr.String())
		}

		//if msg.Flags&proto.FlagPing > 0 {
		//	// todo: may block read loop
		//	c.writeBuffer <- msg.Payload
		//	continue
		//}

		c.sequenceQueue.Push(uint16(echo.Seq))

		if msg.Flags&proto.FlagKeepalive > 0 {
			continue
		}

		c.readBuffer <- msg.Payload
	}
}

func (c *AictConn) writeRoutine() (err error) {
	// wait init
	select {
	case <-c.ctx.Done():
		return nil
	case <-c.initialized:
	}
	log.Println("enter write loop")
	// write loop
	for {
		select {
		case <-c.ctx.Done():
			return nil
		case w := <-c.writeBuffer:
			msg := proto.Layer{
				Flags:   0,
				Payload: w,
			}
			data, err := msg.Marshal()
			if err != nil {
				log.Printf("marshal msg: %v\n", err)
			}
			message := icmp.Message{
				Type: ipv4.ICMPTypeEchoReply,
				Code: 0,
				Body: &icmp.Echo{
					ID:   int(c.identify),
					Seq:  int(c.sequenceQueue.Pop()),
					Data: data,
				},
			}
			raw, err := message.Marshal(nil)
			if err != nil {
				log.Printf("marshal icmp: %v", err)
			}
			_, err = c.conn.WriteTo(raw, c.raddr)
			if err != nil {
				return fmt.Errorf("icmp: write: %v", err)
			}
		}
	}
}

func (c *AictConn) WritePacket(data []byte) error {
	select {
	case <-c.ctx.Done():
		return errors.New("connection closed")
	case c.writeBuffer <- data:
		return nil
	}
}

func (c *AictConn) ReadPacket() ([]byte, error) {
	select {
	case <-c.ctx.Done():
		return nil, errors.New("connection closed")
	case data := <-c.readBuffer:
		return data, nil
	}
}
