package main

import (
	"github.com/BaiMeow/aict/netcfg"
	"golang.zx2c4.com/wireguard/tun"
	"log"
	"net"
	"strings"
)

const MessageTransportOffsetContent = 16

func tunUp(conn Conn, arg string) {
	if arg == "" {
		arg = "tun0"
	}
	device, err := tun.CreateTUN(arg, MTU)
	defer func() {
		log.Println("exit tun, close it")
		err := device.Close()
		if err != nil {
			log.Println("close tun: ", err)
		}
	}()
	if err != nil {
		log.Fatalf("create tun: %v", err)
	}

	_, cidr, err := net.ParseCIDR(address)
	if err != nil {
		log.Fatalf("parse address: %v", err)
	}
	var routesCIDR []*net.IPNet
	for _, route := range strings.Split(routes, ",") {
		_, cidr, err := net.ParseCIDR(route)
		if err != nil {
			log.Fatalf("parse route: %v", err)
		}
		routesCIDR = append(routesCIDR, cidr)
	}

	if err := netcfg.ApplyNet(arg, cidr, routesCIDR); err != nil {
		log.Fatalf("apply net: %v", err)
	}

	go func() {
		evChan := device.Events()
		for {
			ev := <-evChan
			switch ev {
			case tun.EventUp:
				log.Println("tun up")
			case tun.EventDown:
				log.Println("tun down")
			case tun.EventMTUUpdate:
				log.Println("tun mtu update")
			case 0:
				log.Println("tun event channel closed")
				return
			default:
				panic("unhandled default case")
			}
		}
	}()

	batchSize := device.BatchSize()
	rbufs := make([][]byte, batchSize)
	rbufSizes := make([]int, batchSize)
	for i := 0; i < batchSize; i++ {
		rbufs[i] = make([]byte, 1700)
	}
	go func() {
		for {
			n, err := device.Read(rbufs, rbufSizes, MessageTransportOffsetContent)
			if err != nil {
				log.Fatalf("read tun: %v", err)
			}
			for i := 0; i < n; i++ {
				err := conn.WritePacket(rbufs[i][MessageTransportOffsetContent : MessageTransportOffsetContent+rbufSizes[i]])
				if err != nil {
					log.Fatalf("write packet: %v", err)
				}
			}
		}
	}()

	wbuf := make([]byte, 1700)
	for {
		data, err := conn.ReadPacket()
		if err != nil {
			log.Fatalf("read packet: %v", err)
		}
		copy(wbuf[MessageTransportOffsetContent:], data)
		if _, err := device.Write([][]byte{wbuf[:MessageTransportOffsetContent+len(data)]}, MessageTransportOffsetContent); err != nil {
			log.Fatalf("write tun: %v", err)
		}
	}
}
