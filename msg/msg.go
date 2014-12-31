// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package msg defines the main messages used in the Focus protocol.
package msg

import (
	"github.com/mstone/focus/ot"
)

type Cmd int

const (
	C_OPEN Cmd = iota
	C_OPEN_RESP
	C_WRITE
	C_WRITE_RESP
)

type Msg struct {
	Cmd  Cmd
	Name string `json:",omitempty"`
	Fd   int    `json:",omitempty"`
	Rev  int    `json:",omitempty"`
	Ops  ot.Ops `json:",omitempty"`
}
