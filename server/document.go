package server

import (
	"fmt"
	"sync"

	log "gopkg.in/inconshreveable/log15.v2"

	"github.com/mstone/focus/ot"
)

// struct doc represents a vaporpad (like a file)
type doc struct {
	msgs  chan interface{}
	srvr  chan interface{}
	l     log.Logger
	wg    sync.WaitGroup
	name  string
	conns map[int]chan interface{}
	hist  []ot.Ops
	comp  ot.Ops
}

func (d *doc) String() string {
	return fmt.Sprintf("{%s}", d.name)
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

	d.conns[fd] = conn
	d.l.Info("doc received description", "fd", fd)

	m := openresp{
		dbgConn: dbgConn,
		err:     nil,
		doc:     d.msgs,
		fd:      fd,
		name:    d.name,
	}
	d.l.Info("doc sending fd to conn", "action", "SEND", "cmd", m)
	conn <- m

	rev := len(d.hist)
	m2 := write{
		dbgConn: dbgConn,
		fd:      fd,
		rev:     rev,
		ops:     d.comp,
	}
	d.l.Info("doc sending initial write", "action", "SEND", "cmd", m2)
	conn <- m2

	d.l.Info("doc finished opening description", "conn", conn, "fd", fd)
}

func (d *doc) readLoop() {
	defer d.wg.Done()

	for m := range d.msgs {
		d.l.Info("doc got msg", "action", "RECV", "cmd", m)
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
			// if v.ops == nil {
			// 	m := writeresp{
			// 		fd:  v.fd,
			// 		rev: len(d.hist),
			// 	}
			// 	d.l.Info("doc enqueueing FAKE WRITE_RESP", "action", "SEND", "cmd", m)
			// 	d.conns[v.fd] <- m
			// }
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

	// produce transformed ops
	transformedOps := ops
	for _, concurrentOp := range concurrentOps {
		transformedOps, _ = ot.Transform(transformedOps, concurrentOp)
	}
	d.l.Info("doc transformed ops", "cops", concurrentOps, "tops", transformedOps)

	d.hist = append(d.hist, transformedOps)

	// update composed ops for new conns
	prev := d.comp
	d.comp = ot.Compose(d.comp, transformedOps)
	d.l.Info("doc composed transformed ops", "prev", prev, "comp", d.comp)

	rev = len(d.hist)

	return rev, transformedOps
}

func (d *doc) broadcast(conn chan interface{}, fd int, rev int, ops ot.Ops) {
	send := func(pfd int, pconn chan interface{}) {
		if pconn == conn {
			m := writeresp{
				fd:  pfd,
				rev: rev,
			}
			d.l.Info("doc enqueueing WRITE_RESP", "action", "SEND", "cmd", m)
			pconn <- m
		} else {
			m := write{
				fd:  pfd,
				rev: rev,
				ops: ops,
			}
			d.l.Info("doc enqueueing WRITE", "action", "SEND", "cmd", m)
			pconn <- m
		}
	}

	for pfd, pconn := range d.conns {
		send(pfd, pconn)
	}
}
