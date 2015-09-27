package main

import (
	"github.com/mediocregopher/radix.v2/redis"
	//"github.com/mediocregopher/radix.v2/pubsub"
	"fmt"
	//"golang.org/x/net/icmp"
	"errors"
	"golang.org/x/net/ipv6"
	"net"
)

const (
	ProtocolIPv6ICMP = 58
)

var (
	DefaultPrefixLength      byte   = 64
	DefaultValidLifetime     []byte = []byte{0, 0x01, 0x51, 0x80}
	DefaultPreferredLifetime []byte = []byte{0, 0, 0x38, 0x40}
	pc                       *net.PacketConn
	db                       *redis.Client
)

func main() {
	// open redis connection
	redisdb, err := redis.Dial("tcp", "localhost:6379")
	if err != nil {
		panic(err)
	}
	db = redisdb
	defer db.Close()

	// open listening connection
	conn, err := net.ListenPacket("ip6:ipv6-icmp", "::")
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	pc = &conn
	ipconn := ipv6.NewPacketConn(conn)

	filter := new(ipv6.ICMPFilter)
	filter.SetAll(true)
	filter.Accept(ipv6.ICMPTypeRouterSolicitation)
	filter.Accept(ipv6.ICMPTypeRouterAdvertisement)
	filter.Accept(ipv6.ICMPTypeNeighborSolicitation)
	filter.Accept(ipv6.ICMPTypeNeighborAdvertisement)
	filter.Accept(ipv6.ICMPTypeRedirect)
	if err := ipconn.SetICMPFilter(filter); err != nil {
		panic(err)
	}

	// read from socket
	err = nil
	buf := make([]byte, 512)
	var srcAddr net.Addr
	//var body []byte
	var n int
	for err == nil {
		if n, _, srcAddr, err = ipconn.ReadFrom(buf); err != nil {
			fmt.Println(err)
			continue
		}
		addr := srcAddr.(*net.IPAddr)

		go handleND(addr.IP, buf[:n])
		/*
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
		*/
	}
	fmt.Printf("error: %v\n", err)
}

func handleND(src net.IP, body []byte) {
	t := ipv6.ICMPType(body[0])
	fmt.Printf("message from %v type: %v\n", src, t)
	switch t {
	case ipv6.ICMPTypeRouterSolicitation:
		handleRS(src, body)
	case ipv6.ICMPTypeRouterAdvertisement:
		handleRA(src, body)
	case ipv6.ICMPTypeNeighborSolicitation:
		handleNS(src, body)
	case ipv6.ICMPTypeNeighborAdvertisement:
		handleNA(src, body)
	case ipv6.ICMPTypeRedirect:
		handleRedirect(src, body)
	default:
		return
	}
}

type NDOption interface {
	Type() byte
	Marshal() ([]byte, error)
	String() string
}

type NDOptionLLA struct {
	OptionType byte
	Addr       []byte
}

func (o *NDOptionLLA) Type() byte {
	return o.OptionType
}

func (o *NDOptionLLA) Marshal() ([]byte, error) {
	l := len(o.Addr) + 2
	if l%8 != 0 {
		return nil, errors.New("Option length must be multiple of 8")
	}
	return append([]byte{o.OptionType, byte(l / 8)}, o.Addr...), nil
}

func (o *NDOptionLLA) String() string {
	if o.OptionType == 1 {
		return "(src-lla " + net.HardwareAddr(o.Addr).String() + ")"
	}
	if o.OptionType == 2 {
		return "(trg-lla " + net.HardwareAddr(o.Addr).String() + ")"
	}
	return "(" + string(o.OptionType) + ")"
}

func parseOptions(bytes []byte) ([]*NDOption, error) {
	options := make([]*NDOption, 1)
	for len(bytes) > 7 {
		l := int(bytes[1]) * 8
		if l > len(bytes) {
			return options, errors.New("Invalid option length")
		}
		if bytes[0] == 1 || bytes[0] == 2 {
			var option NDOption = &NDOptionLLA{bytes[0], bytes[2:l]}
			options = append(options, &option)
		} else {
			options = append(options, nil)
		}
		bytes = bytes[l:]
	}
	return options, nil
}

func handleRS(src net.IP, body []byte) {
	options, err := parseOptions(body[8:])
	if err != nil {
		fmt.Println(err)
	}
	// look up lla
	var lla *NDOptionLLA = nil
	for _, o := range options {
		if o == nil {
			continue
		}
		llaopt, ok := (*o).(*NDOptionLLA)
		if !ok {
			continue
		}
		lla = llaopt
		if int(lla.OptionType) != 1 {
			continue
		}
	}
	if lla == nil {
		return
	}
	fmt.Println("looking up prefix for " + net.HardwareAddr(lla.Addr).String() + " ...")
	mackey := append([]byte("fahrrad/mac/"), lla.Addr...)
	prefix, err := db.Cmd("GET", mackey).Bytes()
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(prefix) != 16 {
		fmt.Printf("invalid length of prefix %x\n", prefix)
		return
	}
	fmt.Println("found prefix " + net.IP(prefix).String() + "/64")
}

func handleRA(src net.IP, body []byte) {
	_, err := parseOptions(body[16:])
	if err != nil {
		fmt.Println(err)
	}
}

func handleNS(src net.IP, body []byte) {
	_, err := parseOptions(body[24:])
	if err != nil {
		fmt.Println(err)
	}
}

func handleNA(src net.IP, body []byte) {
	_, err := parseOptions(body[24:])
	if err != nil {
		fmt.Println(err)
	}
}

func handleRedirect(src net.IP, body []byte) {
	_, err := parseOptions(body[40:])
	if err != nil {
		fmt.Println(err)
	}
}
