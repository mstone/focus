// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package msg defines the main messages used in the Focus protocol.
// See docs/protocol.adoc for further protocol description.
package msg

import (
	"github.com/mstone/focus/ot"
)

type Cmd int

const (
	C_NIL Cmd = iota
	C_OPEN
	C_OPEN_RESP
	C_WRITE
	C_WRITE_RESP
)

func (c Cmd) String() string {
	switch c {
	case C_NIL:
		return "NIL"
	case C_OPEN:
		return "OPEN"
	case C_OPEN_RESP:
		return "OPEN_RESP"
	case C_WRITE:
		return "WRITE"
	case C_WRITE_RESP:
		return "WRITE_RESP"
	default:
		panic("unknown cmd type")
	}
}

type Msg struct {
	Cmd  Cmd
	Name string `json:",omitempty"`
	Fd   int    `json:",omitempty"`
	Rev  int    `json:",omitempty"`
	Hash string `json:",omitempty"`
	Ops  ot.Ops `json:",omitempty"`
}
