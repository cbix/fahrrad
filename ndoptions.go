package main

import (
	//"github.com/mediocregopher/radix.v2/pubsub"
	"errors"
	"net"
)

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
