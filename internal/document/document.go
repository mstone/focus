// Copyright 2015 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package document

import (
	im "github.com/mstone/focus/internal/msgs"
	"github.com/mstone/focus/ot"
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
		Ops: d.comp,
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
			rev, ops := d.transform(v.Rev, v.Ops)
			d.broadcast(v.Conn, rev, ops)
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
				Ops: ops,
			}
			pconn <- m
		}
	}

	for pconn, _ := range d.conns {
		send(pconn)
	}
}
