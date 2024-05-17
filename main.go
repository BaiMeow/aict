package main

import (
	"flag"
	"fmt"
	"github.com/BaiMeow/aict/client"
	"github.com/BaiMeow/aict/server"
	"log"
	"net"
	"strings"
	"time"
)

type Conn interface {
	ReadPacket() ([]byte, error)
	WritePacket([]byte) error
}

var (
	seqQueueSize int
	clientMode   bool
	serverMode   bool
	local        string
	remote       string
	pipe         string
	MTU          int
	address      string
	routes       string
)

func main() {
	flag.BoolVar(&clientMode, "c", false, "run as client")
	flag.BoolVar(&serverMode, "s", false, "run as server")
	flag.StringVar(&local, "l", "0.0.0.0", "listen addr")
	flag.StringVar(&remote, "r", "0.0.0.0", "remote addr")
	flag.IntVar(&seqQueueSize, "seqQueueSize", 10, "[server mode] size of sequence queue")
	flag.StringVar(&pipe, "p", "tun", "pipe packet, example (udp:12345,tun:tun0)")
	flag.IntVar(&MTU, "mtu", 1280, "[tun] mtu of tun device")
	flag.StringVar(&address, "addr", "", "[tun] interface address must be in CIDR format")
	flag.StringVar(&routes, "routes", "", "[tun] routes,example (1.1.1.1/32,2.2.2.0/30)")
	flag.Parse()

	localAddr := net.ParseIP(local)
	if localAddr == nil {
		log.Fatalln("invalid local addr")
	}
	remoteAddr := net.ParseIP(remote)
	if remoteAddr == nil {
		log.Fatalln("invalid remote addr")
	}

	var (
		conn Conn
		err  error
	)
	if !clientMode && serverMode {
		conn, err = server.Listen(&net.IPAddr{IP: localAddr}, &net.IPAddr{IP: remoteAddr}, &server.Config{SeqQueueSize: seqQueueSize})
		if err != nil {
			log.Fatalf("server: %v", err)
		}
	} else if clientMode && !serverMode {
		conn, err = client.Dial(&net.IPAddr{IP: localAddr}, &net.IPAddr{IP: remoteAddr}, &client.Config{})
		if err != nil {
			log.Fatalf("client: %v", err)
		}
	} else {
		log.Fatalln("args conflict, unknown running mode")
	}

	arr := strings.SplitN(strings.TrimSpace(pipe), ":", 2)
	var pipeProto string
	var pipeArg string
	if len(arr) < 1 {
		log.Fatalln("parse arg pipe failed")
	}
	pipeProto = arr[0]
	if len(arr) == 2 {
		pipeArg = arr[1]
	}

	switch pipeProto {
	case "tun":
		tunUp(conn, pipeArg)
	case "udp":
		panic("not implemented")
	case "test":
		test(conn)
	default:
		log.Fatalf("unknown pipe proto: %s", pipeProto)
	}
}

func test(conn Conn) {
	go func() {
		for {
			data, err := conn.ReadPacket()
			if err != nil {
				log.Fatalln(err)
			}
			fmt.Println(string(data))
		}
	}()
	for {
		err := conn.WritePacket([]byte("bbb"))
		if err != nil {
			log.Fatalln(err)
		}
		time.Sleep(time.Second)
	}
}
