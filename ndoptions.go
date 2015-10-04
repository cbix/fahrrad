// Copyright 2015 Florian Hülsmann <fh@cbix.de>

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

// Interface NDOption is an abstraction for ICMPv6 options in Neighbor Discovery (RFC 4861, 4.6)
type NDOption interface {
	Type() byte               // returns the type field of the option
	Marshal() ([]byte, error) // returns the complete binary representation of the option
	fmt.Stringer
}

// NDOptionLLA is either a source or target link-layer address option (RFC 4861, 4.6.1)
type NDOptionLLA struct {
	OptionType byte
	Addr       net.HardwareAddr
}

// Method Type implements the Type() method of the NDOption interface and returns 1 (source LLA) or 2 (target LLA)
func (o *NDOptionLLA) Type() byte {
	return o.OptionType
}

// Method Marshal implements the Marshal() method of the NDOption interface
func (o *NDOptionLLA) Marshal() ([]byte, error) {
	l := len(o.Addr) + 2
	if l%8 != 0 {
		return nil, errors.New("Option length must be multiple of 8")
	}
	return append([]byte{o.OptionType, byte(l / 8)}, o.Addr...), nil
}

// String() implements the String method of the fmt.Stringer interface
func (o *NDOptionLLA) String() string {
	s := "("
	if o.OptionType == 1 {
		s += "trg-lla "
	}
	if o.OptionType == 2 {
		s += "src-lla "
	}
	return s + o.Addr.String() + ")"
}

// NDOptionPrefix is the prefix information option (RFC 4861, 4.6.2)
type NDOptionPrefix struct {
	PrefixLength      uint8
	OnLink            bool
	AutoConf          bool
	ValidLifetime     uint32
	PreferredLifetime uint32
	Prefix            net.IP
}

// Method Type implements the Type() method of the NDOption interface and always returns 3
func (o *NDOptionPrefix) Type() byte {
	return 3
}

// Method Marshal implements the Marshal() method of the NDOption interface
func (o *NDOptionPrefix) Marshal() ([]byte, error) {
	if o.PrefixLength > 128 {
		return nil, errors.New("invalid prefix length")
	}

	// type and length
	msg := []byte{3, 4}

	// L/A flags and Reserved1
	var flags byte
	if o.OnLink {
		// set on-link bit
		flags |= 0x80
	}
	if o.AutoConf {
		// set autoconf bit
		flags |= 0x40
	}
	msg = append(msg, byte(o.PrefixLength), flags)

	// Valid Lifetime
	vltbuf := new(bytes.Buffer)
	if err := binary.Write(vltbuf, binary.BigEndian, o.ValidLifetime); err != nil {
		return nil, err
	}
	msg = append(msg, vltbuf.Bytes()...)

	// Preferred Lifetime
	pltbuf := new(bytes.Buffer)
	if err := binary.Write(pltbuf, binary.BigEndian, o.PreferredLifetime); err != nil {
		return nil, err
	}
	msg = append(msg, pltbuf.Bytes()...)

	prefix := o.Prefix.To16()
	if prefix == nil {
		return nil, errors.New("wrong prefix size")
	}

	// Reserved2
	msg = append(msg, 0, 0, 0, 0)

	// Prefix
	return append(msg, prefix...), nil
}

// String() implements the String method of the fmt.Stringer interface
func (o *NDOptionPrefix) String() string {
	return "(prefix " + o.Prefix.String() + "/" + string(o.PrefixLength) + ")"
}

// Method parseOptions parses received ND options (only LLA so far)
func parseOptions(bytes []byte) ([]*NDOption, error) {
	options := make([]*NDOption, 1)
	for len(bytes) > 7 {
		l := int(bytes[1]) * 8
		if l > len(bytes) {
			return options, errors.New("Invalid option length")

		}
		var option NDOption
		switch bytes[0] {
		case 1, 2:
			option = &NDOptionLLA{bytes[0], net.HardwareAddr(bytes[2:l])}
		default:
			option = nil
		}
		options = append(options, &option)
		bytes = bytes[l:]
	}
	return options, nil
}
