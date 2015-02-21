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
                                                  doc ----- allocfd -----> srv
                                                  doc <----  *fd   ------- srv
                     conn  <------  fd  --------  doc
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

type Opencompletion struct {
	Fd  int
	Doc chan interface{}
}

// processed by doc for conn
type Open struct {
	Reply chan Opencompletion
	Conn  chan interface{}
	Name  string
}

type Openresp struct {
	Err  error
	Doc  chan interface{}
	Name string
	Fd   int
}

// processed by Server for doc
type Allocfd struct {
	Reply chan Allocfdresp
}

type Allocfdresp struct {
	Err error
	Fd  int
}

// processed by Server for server
type Allocconn struct {
	Reply chan Allocconnresp
}

type Allocconnresp struct {
	Err error
	No  int
}

type Writeresp struct {
	Fd  int
	Rev int
}

type Write struct {
	Fd  int
	Rev int
	Ops ot.Ops
}

type Readall struct {
	Reply chan Readallresp
}

type Readallresp struct {
	Name string
	Body string
	Rev  int
}
