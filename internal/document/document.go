// Copyright 2015 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package document

import (
	"github.com/juju/errors"
	log "gopkg.in/inconshreveable/log15.v2"

	im "github.com/mstone/focus/internal/msgs"
	"github.com/mstone/focus/ot"
)

// struct doc represents a vaporpad (like a file)
type doc struct {
	msgs    chan interface{}
	srvr    chan interface{}
	store   chan interface{}
	name    string
	storeid int64
	conns   map[chan interface{}]struct{}
	hist    []ot.Ops
	comp    ot.Ops
}

func New(srvr chan interface{}, store chan interface{}, name string) (chan interface{}, error) {
	d := &doc{
		msgs:  make(chan interface{}),
		srvr:  srvr,
		store: store,
		name:  name,
		conns: map[chan interface{}]struct{}{},
		hist:  []ot.Ops{},
		comp:  ot.Ops{},
	}
	go d.readLoop()

	replLoad := make(chan im.Loaddocresp, 1)
	d.store <- im.Loaddoc{
		Reply: replLoad,
		Name:  d.name,
	}
	respLoad := <-replLoad
	if respLoad.Err != nil {
		log.Error("unable to load doc", "err", respLoad.Err)
		return nil, respLoad.Err
	}
	if respLoad.Ok {
		d.storeid = respLoad.StoreId
		d.hist = respLoad.History
		for _, ops := range d.hist {
			comp, err := ot.Compose(d.comp, ops)
			if err != nil {
				log.Error("unable to compose doc hist", "err", err)
				return nil, err
			}
			d.comp = comp
		}
	} else {
		repl := make(chan im.Storedocresp, 1)
		d.store <- im.Storedoc{
			Reply: repl,
			Name:  d.name,
		}
		resp := <-repl
		if resp.Err != nil {
			log.Error("unable to create store doc", "err", resp.Err)
			return nil, resp.Err
		}
		d.storeid = resp.StoreId
	}

	return d.msgs, nil
}

func (d *doc) Body() string {
	doc := ot.NewDoc()
	doc.Apply(d.comp)
	return doc.String()
}

func (d *doc) openDescription(fd int, clientRev int, conn chan interface{}) {
	d.conns[conn] = struct{}{}

	// if serverRev < rev, panic?
	serverRev := len(d.hist)
	opsForClient := ot.Ops{}
	var err error

	if clientRev == 0 {
		opsForClient = d.comp.Clone()
	} else {
		if clientRev < serverRev {
			opsForClient, err = ot.ComposeAll(d.hist[clientRev : serverRev-1])
		}
	}

	m := im.Openresp{
		Err:  err,
		Doc:  d.msgs,
		Fd:   fd,
		Name: d.name,
	}
	conn <- m

	if err == nil {
		m2 := im.Write{
			Doc: d.msgs,
			Rev: serverRev,
			Ops: opsForClient, // danger; commutativity violation?
		}
		conn <- m2
	}
}

func (d *doc) readLoop() {
	for m := range d.msgs {
		switch v := m.(type) {
		default:
			panic("doc read unknown message")
		case im.Open:
			d.openDescription(v.Fd, v.Rev, v.Conn)
		case im.Readall:
			v.Reply <- im.Readallresp{
				Name: d.name,
				Body: d.Body(),
				Rev:  len(d.hist),
			}
		case im.Write:
			// BUG(mistone): need to figure out how to handle transform errors!
			rev, ops, _ := d.transform(v.Rev, v.Ops.Clone())
			// BUG(mistone): need to figure out how to handle store write errors!
			_ = d.record(v.Conn, rev, ops)
			// log15.Info("recv", "obj", "doc", "rev", v.Rev, "hash", v.Hash, "ops", v.Ops, "docrev", len(d.hist), "dochist", d.Body(), "nrev", rev, "tops", ops)
			d.broadcast(v.Conn, rev, ops)
		}
	}
}

func (d *doc) transform(rev int, clientOps ot.Ops) (int, ot.Ops, error) {
	var err error

	// extract concurrent ops
	concurrentServerOps := []ot.Ops{}
	if rev < len(d.hist) {
		concurrentServerOps = d.hist[rev:]
	}

	// BUG(mistone): ot.Transform DOES NOT CALCULATE PUSHOUTS.
	// // compose concurrent ops
	// serverOps := ot.Ops{}
	// for _, concurrentOp := range concurrentServerOps {
	// 	serverOps = ot.Compose(serverOps, concurrentOp)
	// }

	// // produce transformed ops
	// forServer, _ := ot.Transform(clientOps, serverOps)

	clientOps2 := ot.Ops{}
	for _, concurrentOp := range concurrentServerOps {
		clientOps2, _, err = ot.Transform(clientOps, concurrentOp)
		if err != nil {
			return 0, nil, errors.Trace(err)
		}
		clientOps = clientOps2.Clone()
	}
	forServer := clientOps

	// update history
	d.hist = append(d.hist, forServer)

	// update composed ops for new conns
	d.comp, err = ot.Compose(d.comp, forServer)
	if err != nil {
		return 0, nil, errors.Trace(err)
	}

	rev = len(d.hist)

	return rev, forServer, nil
}

func (d *doc) record(conn chan interface{}, rev int, ops ot.Ops) error {
	repl := make(chan im.Storewriteresp, 1)
	d.store <- im.Storewrite{
		Reply: repl,
		DocId: d.storeid,
		// AuthorId: ...
		Rev: rev,
		Ops: ops,
	}
	resp := <-repl
	if resp.Err != nil {
		log.Error("unable to store write", "err", resp.Err)
		return resp.Err
	}
	return nil
}

func (d *doc) broadcast(conn chan interface{}, rev int, ops ot.Ops) {
	send := func(pconn chan interface{}) {
		if pconn == conn {
			m := im.Writeresp{
				Doc: d.msgs,
				Rev: rev,
				Ops: ops.Clone(),
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
