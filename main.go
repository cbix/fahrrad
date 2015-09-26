package main

import (
	"github.com/mediocregopher/radix.v2/redis"
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
	// open redis connection
	db, err := redis.Dial("tcp", "localhost:6379")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	//    db.Cmd("SET", append([]byte("fahrrad/test/"), []byte{0x00, 0xaa, 0xbb}...), []byte("Hello world!"))
	//    db.Cmd("SET", append([]byte("fahrrad/test/"), []byte{0x10, 0x0a, 0xcc}...), []byte("foo bar"))

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
	var srcAddr net.Addr
	var body []byte
	var n int
	for err == nil {
		if n, srcAddr, err = conn.ReadFrom(buf); err != nil {
			continue
		}
		if m, err = icmp.ParseMessage(ProtocolIPv6ICMP, buf); err != nil {
			continue
		}
		if body, err = m.Body.Marshal(ProtocolIPv6ICMP); err != nil {
			continue
		}
		fmt.Printf("%v length %d received from %v:\n%x\n%x\n", m.Type, n, srcAddr, buf[:120], body[:120])
		addr := srcAddr.(*net.IPAddr)
		if addr.IP.IsLinkLocalUnicast() {
			ip := []byte(addr.IP)
			llakey := append([]byte("fahrrad/lla/"), []byte(addr.IP)...)
			mac := []byte{ip[8] ^ 0x02, ip[9], ip[10], ip[13], ip[14], ip[15]}
			mackey := append([]byte("fahrrad/mac/"), mac...)
			db.Cmd("INCR", llakey)
			db.Cmd("INCR", mackey)
		} else {
			fmt.Println(addr, "is no linklocal address")
		}
	}
	fmt.Printf("error: %v\n", err)
}
