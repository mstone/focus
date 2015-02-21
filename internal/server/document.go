package server

import (
	"fmt"
	"sync"

	log "gopkg.in/inconshreveable/log15.v2"

	"github.com/mstone/focus/msg"
	"github.com/mstone/focus/ot"
)

type dconn struct {
	conn    chan interface{}
	dbgConn *conn
}

// struct doc represents a vaporpad (like a file)
type doc struct {
	msgs  chan interface{}
	srvr  chan interface{}
	l     log.Logger
	wg    sync.WaitGroup
	name  string
	conns map[int]dconn
	hist  []ot.Ops
	comp  ot.Ops
}

func (d *doc) String() string {
	return fmt.Sprintf("{%s}", d.name)
}

func (d *doc) Body() string {
	doc := ot.NewDoc()
	doc.Apply(d.comp)
	return doc.String()
}

func (d *doc) Run() {
	d.wg.Add(1)
	go d.readLoop()
	d.wg.Wait()
}

func (d *doc) openDescription(dbgConn *conn, conn chan interface{}) {
	srvrReplyChan := make(chan allocfdresp)
	d.srvr <- allocfd{srvrReplyChan}
	srvrResp := <-srvrReplyChan

	if srvrResp.err != nil {
		conn <- openresp{
			err: srvrResp.err,
		}
		return
	}

	fd := srvrResp.fd

	d.conns[fd] = dconn{conn, dbgConn}

	m := openresp{
		dbgConn: dbgConn,
		err:     nil,
		doc:     d.msgs,
		fd:      fd,
		name:    d.name,
	}
	conn <- m

	rev := len(d.hist)
	m2 := write{
		dbgConn: dbgConn,
		fd:      fd,
		rev:     rev,
		ops:     d.comp,
	}
	conn <- m2
}

func unpackMsg(m interface{}, b ...interface{}) []interface{} {
	switch v := m.(type) {
	case open:
		return append(b, "cmd", "open", "conn", v.dbgConn.String(), "name", v.name)
	case openresp:
		return append(b, "cmd", "openresp", "conn", v.dbgConn.String(), "name", v.name, "fd", v.fd, "err", v.err)
	case write:
		return append(b, "cmd", "write", "conn", v.dbgConn.String(), "fd", v.fd, "rev", v.rev, "ops", v.ops)
	case writeresp:
		return append(b, "cmd", "writeresp", "conn", v.dbgConn.String(), "fd", v.fd, "rev", v.rev)
	case msg.Msg:
		switch v.Cmd {
		case msg.C_WRITE_RESP:
			return append(b, "cmd", "WRITE_RESP", "fd", v.Fd, "rev", v.Rev)
		case msg.C_WRITE:
			return append(b, "cmd", "WRITE", "fd", v.Fd, "rev", v.Rev, "ops", v.Ops)
		case msg.C_OPEN:
			return append(b, "cmd", "OPEN", "name", v.Name)
		case msg.C_OPEN_RESP:
			return append(b, "cmd", "OPEN_RESP", "name", v.Name, "fd", v.Fd)
		}
	}
	return append(b, "cmd", m)
}

func (d *doc) readLoop() {
	defer d.wg.Done()

	for m := range d.msgs {
		switch v := m.(type) {
		default:
			panic("doc read unknown message")
		case open:
			d.openDescription(v.dbgConn, v.conn)
		case write:
			conn, ok := d.conns[v.fd]
			if !ok {
				panic("doc got write with unknown fd")
			}
			rev, ops := d.transform(v.rev, v.ops)
			d.broadcast(conn, v.fd, rev, ops)
		}
	}
}

func (d *doc) transform(rev int, ops ot.Ops) (int, ot.Ops) {
	// extract concurrent ops
	concurrentOps := []ot.Ops{}
	if rev < len(d.hist) {
		concurrentOps = d.hist[rev:]
	}

	// compose concurrent ops
	composedOps := ot.Ops{}
	for _, concurrentOp := range concurrentOps {
		composedOps = ot.Compose(composedOps, concurrentOp)
	}

	// produce transformed ops
	transformedOps, _ := ot.Transform(ops, composedOps)

	// update history
	d.hist = append(d.hist, transformedOps)

	// update composed ops for new conns
	d.comp = ot.Compose(d.comp, transformedOps)

	rev = len(d.hist)

	return rev, transformedOps
}

func (d *doc) broadcast(conn dconn, fd int, rev int, ops ot.Ops) {
	send := func(pfd int, pconn dconn) {
		if pconn == conn {
			m := writeresp{
				dbgConn: pconn.dbgConn,
				fd:      pfd,
				rev:     rev,
			}
			pconn.conn <- m
		} else {
			m := write{
				dbgConn: pconn.dbgConn,
				fd:      pfd,
				rev:     rev,
				ops:     ops,
			}
			pconn.conn <- m
		}
	}

	for pfd, pconn := range d.conns {
		send(pfd, pconn)
	}
}
