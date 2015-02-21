// Copyright 2015 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package server

import (
	"github.com/mstone/focus/ot"
)

/*

Proto:

cl ----  dial  --->  srv
cl <---  *conn ----  srv
cl ----  OPEN ---->  conn
         name
                     conn  ----- Allocdoc ----->  srv
                     conn  <----   *doc  -------  srv
                     conn  -----   open    ---->  doc
                     conn  <------ fd    -------  doc
                     conn  <------ openresp ----  doc
                     conn  <------ write  ------  doc
cl <-- openresp  --  conn
cl <--  write -----  conn

*/

// processed by Server for conn
type Allocdoc struct {
	Reply chan Allocdocresp
	Name  string
}

type Allocdocresp struct {
	Err error
	Doc chan interface{}
}

// processed by doc for conn
type Open struct {
	Conn chan interface{}
	Name string
	Fd   int
}

type Openresp struct {
	Err  error
	Doc  chan interface{}
	Name string
	Fd   int
}

// processed by conn for doc
type Writeresp struct {
	Doc chan interface{}
	Rev int
}

// processed by doc for conn and by conn for doc
type Write struct {
	Conn chan interface{}
	Doc  chan interface{}
	Rev  int
	Ops  ot.Ops
}

// processed by doc for tests
type Readall struct {
	Reply chan Readallresp
}

type Readallresp struct {
	Name string
	Body string
	Rev  int
}
