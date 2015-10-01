// Copyright 2015 Florian Hülsmann <fh@cbix.de>

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

type NDOption interface {
	Type() byte
	Marshal() ([]byte, error)
	fmt.Stringer
}

type NDOptionLLA struct {
	OptionType byte
	Addr       net.HardwareAddr
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

// String() implements the String method of the fmt.Stringer interface
func (o *NDOptionLLA) String() string {
	if o.OptionType == 1 {
		return "(src-lla " + o.Addr.String() + ")"
	}
	if o.OptionType == 2 {
		return "(trg-lla " + o.Addr.String() + ")"
	}
	return "(" + string(o.OptionType) + ")"
}

type NDOptionPrefix struct {
	PrefixLength      uint8
	OnLink            bool
	AutoConf          bool
	ValidLifetime     uint32
	PreferredLifetime uint32
	Prefix            net.IP
}

func (o *NDOptionPrefix) Type() byte {
	return 3
}

func (o *NDOptionPrefix) Marshal() ([]byte, error) {
	msg := []byte{3, 4}
	if o.PrefixLength > 128 {
		return nil, errors.New("invalid prefix length")
	}
	var flags byte
	if o.OnLink {
		flags |= 0x80
	}
	if o.AutoConf {
		flags |= 0x40
	}
	msg = append(msg, byte(o.PrefixLength), flags)
	// marshal lifetime values
	vltbuf := new(bytes.Buffer)
	if err := binary.Write(vltbuf, binary.BigEndian, o.ValidLifetime); err != nil {
		return nil, err
	}
	msg = append(msg, vltbuf.Bytes()...)
	pltbuf := new(bytes.Buffer)
	if err := binary.Write(pltbuf, binary.BigEndian, o.PreferredLifetime); err != nil {
		return nil, err
	}
	msg = append(msg, pltbuf.Bytes()...)
	prefix := o.Prefix.To16()
	if prefix == nil {
		return nil, errors.New("wrong prefix size")
	}
	msg = append(msg, 0, 0, 0, 0)
	return append(msg, prefix...), nil
}

func (o *NDOptionPrefix) String() string {
	return "(prefix " + o.Prefix.String() + "/" + string(o.PrefixLength) + ")"
}

func parseOptions(bytes []byte) ([]*NDOption, error) {
	options := make([]*NDOption, 1)
	for len(bytes) > 7 {
		l := int(bytes[1]) * 8
		if l > len(bytes) {
			return options, errors.New("Invalid option length")

		}
		if bytes[0] == 1 || bytes[0] == 2 {
			var option NDOption = &NDOptionLLA{bytes[0], net.HardwareAddr(bytes[2:l])}
			options = append(options, &option)
		} else {
			options = append(options, nil)
		}
		bytes = bytes[l:]
	}
	return options, nil
}
