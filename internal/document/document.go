package document

import (
	"sync"

	log "gopkg.in/inconshreveable/log15.v2"

	im "github.com/mstone/focus/internal/msgs"
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

func New(srvr chan interface{}, name string) chan interface{} {
	d := &doc{
		msgs:  make(chan interface{}),
		srvr:  srvr,
		wg:    sync.WaitGroup{},
		name:  name,
		conns: map[int]chan interface{}{},
		hist:  []ot.Ops{},
		comp:  ot.Ops{},
	}
	go d.Run()
	return d.msgs
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

func (d *doc) openDescription(conn chan interface{}, reply chan im.Opencompletion) {
	srvrReplyChan := make(chan im.Allocfdresp)
	d.srvr <- im.Allocfd{srvrReplyChan}
	srvrResp := <-srvrReplyChan

	if srvrResp.Err != nil {
		conn <- im.Openresp{
			Err: srvrResp.Err,
		}
		return
	}

	fd := srvrResp.Fd

	reply <- im.Opencompletion{fd, d.msgs}
	close(reply)

	d.conns[fd] = conn

	m := im.Openresp{
		Err:  nil,
		Doc:  d.msgs,
		Fd:   fd,
		Name: d.name,
	}
	conn <- m

	rev := len(d.hist)
	m2 := im.Write{
		Fd:  fd,
		Rev: rev,
		Ops: d.comp,
	}
	conn <- m2
}

func (d *doc) readLoop() {
	defer d.wg.Done()

	for m := range d.msgs {
		switch v := m.(type) {
		default:
			panic("doc read unknown message")
		case im.Open:
			d.openDescription(v.Conn, v.Reply)
		case im.Readall:
			v.Reply <- im.Readallresp{
				Name: d.name,
				Body: d.Body(),
				Rev:  len(d.hist),
			}
		case im.Write:
			conn, ok := d.conns[v.Fd]
			if !ok {
				panic("doc got write with unknown fd")
			}
			rev, ops := d.transform(v.Rev, v.Ops)
			d.broadcast(conn, v.Fd, rev, ops)
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

func (d *doc) broadcast(conn chan interface{}, fd int, rev int, ops ot.Ops) {
	send := func(pfd int, pconn chan interface{}) {
		if pconn == conn {
			m := im.Writeresp{
				Fd:  pfd,
				Rev: rev,
			}
			pconn <- m
		} else {
			m := im.Write{
				Fd:  pfd,
				Rev: rev,
				Ops: ops,
			}
			pconn <- m
		}
	}

	for pfd, pconn := range d.conns {
		send(pfd, pconn)
	}
}
