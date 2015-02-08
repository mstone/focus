package server

import (
	"fmt"

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
cl <----  fd  -----  conn

*/

// processed by Server for conn
type allocdoc struct {
	reply chan allocdocresp
	name  string
}

type allocdocresp struct {
	err error
	doc chan interface{}
}

// processed by doc for conn
type open struct {
	dbgConn *conn
	conn    chan interface{}
	name    string
}

func (o open) String() string {
	return fmt.Sprintf("open{conn: %s, name: %s}", o.dbgConn, o.name)
}

type openresp struct {
	err     error
	dbgConn *conn
	doc     chan interface{}
	name    string
	fd      int
}

func (o openresp) String() string {
	errstr := "nil"
	if o.err != nil {
		errstr = o.err.Error()
	}
	return fmt.Sprintf("openresp{conn: %s, doc: <>, name: %s, fd: %d, err: %s}", o.dbgConn, o.name, o.fd, errstr)
}

// processed by Server for doc
type allocfd struct {
	reply chan allocfdresp
}

type allocfdresp struct {
	err error
	fd  int
}

// processed by Server for server
type allocconn struct {
	reply chan allocconnresp
}

type allocconnresp struct {
	err error
	no  int
}

type writeresp struct {
	dbgConn *conn
	fd      int
	rev     int
}

func (w writeresp) String() string {
	return fmt.Sprintf("writeresp{conn: %s, fd: %d, rev: %d}", w.dbgConn, w.fd, w.rev)
}

type write struct {
	dbgConn *conn
	fd      int
	rev     int
	ops     ot.Ops
}

func (w write) String() string {
	return fmt.Sprintf("write{conn: %s, fd: %d, rev: %d, ops: %s}", w.dbgConn, w.fd, w.rev, w.ops)
}
