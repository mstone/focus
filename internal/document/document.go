// Copyright 2015 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package document

import (
	im "github.com/mstone/focus/internal/msgs"
	"github.com/mstone/focus/ot"
	"gopkg.in/inconshreveable/log15.v2"
)

// struct doc represents a vaporpad (like a file)
type doc struct {
	msgs  chan interface{}
	srvr  chan interface{}
	name  string
	conns map[chan interface{}]struct{}
	hist  []ot.Ops
	comp  ot.Ops
}

func New(srvr chan interface{}, name string) chan interface{} {
	d := &doc{
		msgs:  make(chan interface{}),
		srvr:  srvr,
		name:  name,
		conns: map[chan interface{}]struct{}{},
		hist:  []ot.Ops{},
		comp:  ot.Ops{},
	}
	go d.readLoop()
	return d.msgs
}

func (d *doc) Body() string {
	doc := ot.NewDoc()
	doc.Apply(d.comp)
	return doc.String()
}

func (d *doc) openDescription(fd int, conn chan interface{}) {
	d.conns[conn] = struct{}{}

	m := im.Openresp{
		Err:  nil,
		Doc:  d.msgs,
		Fd:   fd,
		Name: d.name,
	}
	conn <- m

	rev := len(d.hist)
	m2 := im.Write{
		Doc: d.msgs,
		Rev: rev,
		Ops: d.comp.Clone(),
	}
	conn <- m2
}

func (d *doc) readLoop() {
	for m := range d.msgs {
		switch v := m.(type) {
		default:
			panic("doc read unknown message")
		case im.Open:
			d.openDescription(v.Fd, v.Conn)
		case im.Readall:
			v.Reply <- im.Readallresp{
				Name: d.name,
				Body: d.Body(),
				Rev:  len(d.hist),
			}
		case im.Write:
			rev, ops := d.transform(v.Rev, v.Ops.Clone())
			log15.Info("recv", "obj", "doc", "rev", v.Rev, "ops", v.Ops, "docrev", len(d.hist), "dochist", d.Body(), "nrev", rev, "tops", ops)
			d.broadcast(v.Conn, rev, ops)
		}
	}
}

func (d *doc) transform(rev int, clientOps ot.Ops) (int, ot.Ops) {
	// extract concurrent ops
	concurrentServerOps := []ot.Ops{}
	if rev < len(d.hist) {
		concurrentServerOps = d.hist[rev:]
	}

	// compose concurrent ops
	serverOps := ot.Ops{}
	for _, concurrentOp := range concurrentServerOps {
		serverOps = ot.Compose(serverOps, concurrentOp)
	}

	// produce transformed ops
	forServer, _ := ot.Transform(clientOps, serverOps)

	// update history
	// d.hist = append(d.hist, transformedOps)
	d.hist = append(d.hist, forServer)

	// update composed ops for new conns
	d.comp = ot.Compose(d.comp, forServer)

	rev = len(d.hist)

	return rev, forServer
}

func (d *doc) broadcast(conn chan interface{}, rev int, ops ot.Ops) {
	send := func(pconn chan interface{}) {
		if pconn == conn {
			m := im.Writeresp{
				Doc: d.msgs,
				Rev: rev,
			}
			pconn <- m
		} else {
			m := im.Write{
				Doc: d.msgs,
				Rev: rev,
				Ops: ops.Clone(),
			}
			pconn <- m
		}
	}

	for pconn, _ := range d.conns {
		send(pconn)
	}
}
