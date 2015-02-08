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
	d.l.Info("new doc running")
	d.wg.Add(1)
	go d.readLoop()
	d.wg.Wait()
}

func (d *doc) openDescription(dbgConn *conn, conn chan interface{}) {
	d.l.Info("doc opening description", "conn", conn)

	srvrReplyChan := make(chan allocfdresp)
	d.srvr <- allocfd{srvrReplyChan}
	srvrResp := <-srvrReplyChan

	if srvrResp.err != nil {
		d.l.Error("doc unable to allocfd; sending err to conn", "err", srvrResp.err)
		conn <- openresp{
			err: srvrResp.err,
		}
		return
	}

	fd := srvrResp.fd

	d.conns[fd] = dconn{conn, dbgConn}
	d.l.Info("doc received description", "fd", fd)

	m := openresp{
		dbgConn: dbgConn,
		err:     nil,
		doc:     d.msgs,
		fd:      fd,
		name:    d.name,
	}
	args := unpackMsg(m, "action", "SEND")
	d.l.Info("doc sending fd to conn", args...)
	conn <- m

	rev := len(d.hist)
	m2 := write{
		dbgConn: dbgConn,
		fd:      fd,
		rev:     rev,
		ops:     d.comp,
	}
	args2 := unpackMsg(m2, "action", "SEND")
	d.l.Info("doc sending initial write", args2...)
	conn <- m2

	d.l.Info("doc finished opening description", "conn", conn, "fd", fd)
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
		// d.l.Info("doc got msg", "action", "RECV", "cmd", m)
		args := unpackMsg(m, "action", "RECV")
		d.l.Info("doc got msg", args...)
		switch v := m.(type) {
		default:
			d.l.Error("doc read unknown message", "cmd", v)
			panic("doc read unknown message")
		case open:
			d.openDescription(v.dbgConn, v.conn)
		case write:
			conn, ok := d.conns[v.fd]
			if !ok {
				d.l.Error("doc got write with unknown fd, exiting")
				panic("doc got write with unknown fd")
			}
			d.l.Info("doc transforming", "cmd", v)
			rev, ops := d.transform(v.rev, v.ops)
			d.l.Info("doc broadcasting", "cmd", v)
			d.broadcast(conn, v.fd, rev, ops)
		}
	}
}

func (d *doc) transform(rev int, ops ot.Ops) (int, ot.Ops) {
	d.l.Info("doc transforming ops")

	// extract concurrent ops
	concurrentOps := []ot.Ops{}
	if rev < len(d.hist) {
		concurrentOps = d.hist[rev:]
	}
	d.l.Info("doc found concurrent ops-lists", "num", len(concurrentOps), "val", concurrentOps)

	// compose concurrent ops
	composedOps := ot.Ops{}
	for _, concurrentOp := range concurrentOps {
		composedOps = ot.Compose(composedOps, concurrentOp)
	}
	d.l.Info("doc composed concurrent ops", "num", len(composedOps), "val", composedOps)

	// produce transformed ops
	transformedOps, _ := ot.Transform(ops, composedOps)
	d.l.Info("doc transformed ops", "action", "XFRM", "ops", ops, "cops", composedOps, "tops", transformedOps)

	d.hist = append(d.hist, transformedOps)

	// update composed ops for new conns
	prev := d.comp
	d.comp = ot.Compose(d.comp, transformedOps)
	d.l.Info("doc composed transformed ops", "action", "COMP", "prev", prev, "comp", d.comp)

	rev = len(d.hist)
	d.l.Info("doc state", "action", "STAT", "rev", rev, "comp", d.comp, "hist", d.hist, "body", d.Body())

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
			args := unpackMsg(m, "action", "SEND")
			d.l.Info("doc enqueueing WRITE_RESP", args...)
			pconn.conn <- m
		} else {
			m := write{
				dbgConn: pconn.dbgConn,
				fd:      pfd,
				rev:     rev,
				ops:     ops,
			}
			args := unpackMsg(m, "action", "SEND")
			d.l.Info("doc enqueueing WRITE", args...)
			pconn.conn <- m
		}
	}

	for pfd, pconn := range d.conns {
		send(pfd, pconn)
	}
}
