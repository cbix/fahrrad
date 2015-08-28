package main

import (
	//"github.com/mediocregopher/radix.v2/redis"
	//"github.com/mediocregopher/radix.v2/pubsub"
	"fmt"
	"golang.org/x/net/icmp"
	"net"
	//"golang.org/x/net/ipv6"
)

const (
	ProtocolIPv6ICMP = 58
)

func main() {
	// open listening connection
	conn, err := icmp.ListenPacket("ip6:ipv6-icmp", "::")
	if err != nil {
		fmt.Println(err)
	}
	defer conn.Close()

	// read from socket
	err = nil
	buf := make([]byte, 512)
	var m *icmp.Message
	var addr net.Addr
    var body []byte
    var n int
	for err == nil {
		if n, addr, err = conn.ReadFrom(buf); err != nil {
			continue
		}
		if m, err = icmp.ParseMessage(ProtocolIPv6ICMP, buf); err != nil {
			continue
		}
        if body, err = m.Body.Marshal(ProtocolIPv6ICMP); err != nil {
            continue
        }

        fmt.Printf("%v received from %v: %x\n", m.Type, addr, body[:n])
	}
    fmt.Printf("error: %v\n", err)
}
