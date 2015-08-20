package main

import (
//	"github.com/garyburd/redigo/redis"
	"golang.org/x/net/icmp"
//	"golang.org/x/net/ipv6"
    "fmt"
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
    for err == nil {
        n, addr, err := conn.ReadFrom(buf)
        fmt.Printf("length: %d, address: %v, type: %d, error: %v\n", n, addr, buf[0], err)
    }

}
