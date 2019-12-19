// Copyright 2015 Florian Hülsmann <fh@cbix.de>

package main

import (
	"fmt"
	"github.com/mediocregopher/radix.v2/pool"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
	"net"
	"time"
)

type routerSolicitation struct {
	ip  net.Addr
	mac net.HardwareAddr
}

var (
	pc     *ipv6.PacketConn
	db     *pool.Pool
	rschan chan routerSolicitation
	// default config, might be overwritten by redis hash key fahrrad/config
	AssignedPrefixLength     uint8          = 64
	OnLinkPrefixLength       uint8          = 48
	DefaultValidLifetime     uint32         = 86400
	DefaultPreferredLifetime uint32         = 14400
	TickerDelay              time.Duration  = 5 * time.Minute
	defaultConfig            map[string]int = map[string]int{
		"AssignedPrefixLength":     int(AssignedPrefixLength),
		"OnLinkPrefixLength":       int(OnLinkPrefixLength),
		"DefaultValidLifetime":     int(DefaultValidLifetime),
		"DefaultPreferredLifetime": int(DefaultPreferredLifetime),
		"TickerDelay":              int(TickerDelay / time.Second),
	}
)

func main() {
	var err error
	// create redis connection pool
	if db, err = pool.New("tcp", "localhost:6379", 10); err != nil {
		panic(err)
	}
	defer db.Empty()
	dbc, err := db.Get()
	if err != nil {
		fmt.Println(err)
	}
	for k, v := range defaultConfig {
		dbc.PipeAppend("HSETNX", "fahrrad/config", k, v)
	}
	for k, _ := range defaultConfig {
		dbc.PipeAppend("HGET", "fahrrad/config", k)
	}
	for _, _ = range defaultConfig {
		dbc.PipeResp()
	}
	var v int
	v, err = dbc.PipeResp().Int()
	if err == nil {
		AssignedPrefixLength = uint8(v)
	}
	v, err = dbc.PipeResp().Int()
	if err == nil {
		OnLinkPrefixLength = uint8(v)
	}
	v, err = dbc.PipeResp().Int()
	if err == nil {
		DefaultValidLifetime = uint32(v)
	}
	v, err = dbc.PipeResp().Int()
	if err == nil {
		DefaultPreferredLifetime = uint32(v)
	}
	v, err = dbc.PipeResp().Int()
	if err == nil {
		TickerDelay = time.Duration(v) * time.Second
	}
	defer db.Put(dbc)

	// open listening connection
	conn, err := net.ListenIP("ip6:ipv6-icmp", &net.IPAddr{net.IPv6unspecified, ""})
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	pc = ipv6.NewPacketConn(conn)
	// RFC4861 requires the hop limit set to 255, but the default value in golang is 64
	pc.SetHopLimit(255)

	// only accept neighbor discovery messages
	filter := new(ipv6.ICMPFilter)
	filter.SetAll(true)
	filter.Accept(ipv6.ICMPTypeRouterSolicitation)
	filter.Accept(ipv6.ICMPTypeRouterAdvertisement)
	filter.Accept(ipv6.ICMPTypeNeighborSolicitation)
	filter.Accept(ipv6.ICMPTypeNeighborAdvertisement)
	if err = pc.SetICMPFilter(filter); err != nil {
		panic(err)
	}

	rschan = make(chan routerSolicitation)
	go hostManager()

	// read from socket
	buf := make([]byte, 512)
	for {
		n, _, srcAddr, err := pc.ReadFrom(buf)
		if err != nil {
			panic(err)
		}
		go handleND(srcAddr, buf[:n])
	}
}

// method hostManager holds a list of hosts that get RAs periodically
func hostManager() {
	var activeRAs map[string]routerSolicitation = make(map[string]routerSolicitation)
	var tick *time.Ticker = time.NewTicker(TickerDelay)
	for {
		select {
		case rs := <-rschan:
			k := string(rs.ip.(*net.IPAddr).IP)
			activeRAs[k] = rs
			success := handleRS(rs)
			if !success {
				delete(activeRAs, k)
			}
		case <-tick.C:
			for _, rs := range activeRAs {
				handleRS(rs)
			}
		}
	}
}

// method handleND parses arbitrary ICMPv6 messages, currently only router solicitations
func handleND(src net.Addr, body []byte) {
	t := ipv6.ICMPType(body[0])
	fmt.Printf("%v from %v\n", t, src)
	switch t {
	case ipv6.ICMPTypeRouterSolicitation:
		// parse ND options
		options, err := parseOptions(body[8:])
		if err != nil {
			fmt.Println(err)
		}

		// check if any of the options is a source LLA
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
			fmt.Println("no source LLA option given")
			return
		}
		lladdr := make(net.HardwareAddr, len(lla.Addr))
		copy(lladdr, lla.Addr)
		rschan <- routerSolicitation{src, lladdr}
	default:
		return
	}
}

// method handleRS takes a router solicitation and eventually replies with a unicasted router advertisement
func handleRS(rs routerSolicitation) bool {
	// lookup prefix from redis
	dbc, err := db.Get()
	if err != nil {
		fmt.Println(err)
	}
	defer db.Put(dbc)
	fmt.Printf("looking up prefix for %v ... ", net.HardwareAddr(rs.mac))
	mackey := append([]byte("fahrrad/mac/"), rs.mac...)
	prefix, err := dbc.Cmd("GET", mackey).Bytes()
	if err != nil {
		//fmt.Println(err)
		// i.e. key doesn't exist
		fmt.Printf("not found: %v\n", err)
		return false
	}
	if len(prefix) != 16 {
		fmt.Printf("invalid length: %x\n", prefix)
		return false
	}
	// read config from db
	var v int
	config := make(map[string]int)

	for k, dv := range defaultConfig {
        v, err = dbc.Cmd("HGET", "fahrrad/config", k).Int()
        if err != nil {
            v = dv
        }
        config[k] = v
	}

	fmt.Printf("found %v/%d\n", net.IP(prefix), config["AssignedPrefixLength"])
	// ICMPv6 RA header:
	msgbody := []byte{0x40, 0x00, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	/*
	   According to RFC 5942, the announced prefixes for on-link usage and autoconfiguration
	   can be separate from each other (there can be an arbitrary number of advertised on-link
	   and autoconf prefixes, respectively).
	   This allows us to use /48 for the link but tell the clients to use /64 for autoconf.
	*/
	// Prefix options:
	// autoconf (assigned prefix) option
	apopt := &NDOptionPrefix{
		PrefixLength:      uint8(config["AssignedPrefixLength"]),
		OnLink:            false,
		AutoConf:          true,
		ValidLifetime:     uint32(config["DefaultValidLifetime"]),
		PreferredLifetime: uint32(config["DefaultPreferredLifetime"]),
		Prefix:            net.IP(prefix).Mask(net.CIDRMask(config["AssignedPrefixLength"], 128)),
	}
	// onlink prefix option
	olopt := &NDOptionPrefix{
		PrefixLength:      uint8(config["OnLinkPrefixLength"]),
		OnLink:            true,
		AutoConf:          false,
		ValidLifetime:     uint32(config["DefaultValidLifetime"]),
		PreferredLifetime: uint32(config["DefaultPreferredLifetime"]),
		Prefix:            net.IP(prefix).Mask(net.CIDRMask(int(OnLinkPrefixLength), 128)),
	}
	apoptbytes, err := apopt.Marshal()
	if err != nil {
		// this should never happen
		fmt.Println(err)
		return false
	}
	msgbody = append(msgbody, apoptbytes...)
	oloptbytes, err := olopt.Marshal()
	if err != nil {
		// this should never happen
		fmt.Println(err)
		return false
	}
	msgbody = append(msgbody, oloptbytes...)

	// at this point we could include a source LLA option, but RFC 4861 doesn't require that
	// this would work by taking net.InterfaceByName(src.(*net.IPAddr).Zone) and its HardwareAddr

	// code and checksum are 0, the latter is calculated by the kernel
	// TODO: data structure for RA/ND message body
	msg := &icmp.Message{ipv6.ICMPTypeRouterAdvertisement, 0, 0, &icmp.DefaultMessageBody{msgbody}}
	mb, err := msg.Marshal(nil)
	if err != nil {
		panic(err)
	}
	// send package
	_, err = pc.WriteTo(mb, nil, rs.ip)
	if err != nil {
		panic(err)
	}
	return true
}
