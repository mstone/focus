// Copyright 2015 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package server

import (
	"encoding/json"
	"flag"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	log "gopkg.in/inconshreveable/log15.v2"

	"github.com/mstone/focus/internal/connection"
	im "github.com/mstone/focus/internal/msgs"
	"github.com/mstone/focus/msg"
	"github.com/mstone/focus/ot"
	"github.com/mstone/focus/store"
)

const numClients = 4
const numRounds = 10
const numChars = 10
const readTimeout = 100 * time.Millisecond
const writeTimeout = 500 * time.Millisecond

// const numClients = 3
// const numRounds = 3
// const numChars = 3
// const readTimeout = 50 * time.Millisecond
// const writeTimeout = 500 * time.Millisecond

type ws struct {
	rq, wq chan interface{}
	rt, wt *time.Timer
	rm, wm sync.Mutex
}

func NewWSPair() (*ws, *ws) {
	q1 := make(chan interface{}, numClients*numRounds*2)
	q2 := make(chan interface{}, numClients*numRounds*2)
	// q1 := make(chan interface{}, 0)
	// q2 := make(chan interface{}, 0)
	w1 := &ws{
		rq: q1,
		wq: q2,
		rt: time.NewTimer(readTimeout),
		wt: time.NewTimer(writeTimeout),
		rm: sync.Mutex{},
		wm: sync.Mutex{},
	}
	w2 := &ws{
		rq: q2,
		wq: q1,
		rt: time.NewTimer(readTimeout),
		wt: time.NewTimer(writeTimeout),
		rm: sync.Mutex{},
		wm: sync.Mutex{},
	}
	w1.rt.Stop()
	w1.wt.Stop()
	w2.rt.Stop()
	w2.wt.Stop()
	return w1, w2
}

func (w *ws) ReadJSON(v interface{}) error {
	w.rm.Lock()
	defer w.rm.Unlock()

	select {
	case <-w.rt.C:
		return fmt.Errorf("ws read timeout")
	case v2 := <-w.rq:
		js, _ := json.Marshal(v2)
		return json.Unmarshal(js, v)
	}
}

func (w *ws) WriteJSON(v interface{}) error {
	w.wm.Lock()
	defer w.wm.Unlock()

	select {
	case <-w.wt.C:
		return fmt.Errorf("ws write timeout")
	case w.wq <- v:
		return nil
	}
}

func (w *ws) SetReadTimeout(d time.Duration) error {
	w.rm.Lock()
	defer w.rm.Unlock()

	w.rt.Stop()
	w.rt = time.NewTimer(d)
	return nil
}

func (w *ws) SetWriteTimeout(d time.Duration) error {
	w.wm.Lock()
	defer w.wm.Unlock()

	w.wt.Stop()
	w.wt = time.NewTimer(d)
	return nil
}

func (w *ws) CancelReadTimeout() error {
	w.rm.Lock()
	defer w.rm.Unlock()

	w.rt.Stop()
	return nil
}

func (w *ws) CancelWriteTimeout() error {
	w.wm.Lock()
	defer w.wm.Unlock()

	w.wt.Stop()
	return nil
}

type client struct {
	clname  string
	name    string
	ws      connection.WebSocket
	doc     *ot.Doc
	st      *ot.Controller
	numSend int
	numRecv int
	l       log.Logger
	hist    []msg.Msg
}

func (c *client) sendRandomOps() {
	// size := c.doc.Len()
	ops := c.doc.GetRandomOps(numChars)

	c.doc.Apply(ops)
	c.st.OnClientWrite(ops.Clone())
	// c.l.Info("genn", "ops", ops, "docsize", size, "doc", c.doc.String(), "docp", fmt.Sprintf("%p", c.doc), "clnhist", c.doc.String(), "clnst", c.st)
}

func (c *client) String() string {
	return fmt.Sprintf("%s", c.clname)
}

func (c *client) Send(rev int, hash string, ops ot.Ops) {
	c.ws.SetWriteTimeout(writeTimeout)
	m := msg.Msg{
		Cmd:  msg.C_WRITE,
		Rev:  rev,
		Hash: hash,
		Ops:  ops.Clone(),
	}
	// c.l.Info("send", "num", c.numSend, "rev", rev, "ops", ops)
	err := c.ws.WriteJSON(m)
	c.ws.CancelWriteTimeout()
	if err != nil {
		panic("client unable to send WRITE: " + err.Error())
	}
	c.numSend++
}

func (c *client) Recv(ops ot.Ops) {
	c.doc.Apply(ops.Clone())
	// c.l.Info("stat", "body", c.doc.String(), "clnst", c.st)
}

func (c *client) onWriteResp(m msg.Msg) {
	// c.l.Info("recv", "num", c.numRecv, "kind", "ack1", "rev", m.Rev, "ops", m.Ops, "clnhist", c.doc.String(), "clnst", c.st)
	c.st.OnServerAck(m.Rev, m.Ops)
	// c.l.Info("recv", "num", c.numRecv, "kind", "ack2", "rev", m.Rev, "ops", m.Ops, "clnhist", c.doc.String(), "clnst", c.st)
}

func (c *client) onWrite(m msg.Msg) {
	// c.l.Info("recv", "num", c.numRecv, "kind", "wrt1", "rev", m.Rev, "ops", m.Ops, "clnhist", c.doc.String(), "clnst", c.st)
	c.st.OnServerWrite(m.Rev, m.Ops.Clone())
	// c.l.Info("recv", "num", c.numRecv, "kind", "wrt2", "rev", m.Rev, "ops", m.Ops, "clnhist", c.doc.String(), "clnst", c.st)
}

func (c *client) loop() {
	round := 0
	// for round = 0; round < numRounds; round++ {
	// 	c.sendRandomOps()
	// }

Loop:
	for {
		if round < numRounds {
			c.sendRandomOps()
			round++
		}

		m := msg.Msg{}
		c.ws.SetReadTimeout(readTimeout)
		err := c.ws.ReadJSON(&m)
		c.ws.CancelReadTimeout()
		if err != nil {
			log.Error("client unable to read response", "err", err)
			break Loop
		}
		switch m.Cmd {
		case msg.C_WRITE_RESP:
			c.onWriteResp(m)
		case msg.C_WRITE:
			c.onWrite(m)
		}
		c.numRecv++
	}
}

func testOnce(t *testing.T, iteration int) {
	var err error
	log.Crit("boot")

	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		log.Crit("unable to open driver", "err", err)
		return
	}

	focusStore := store.New(db)

	err = focusStore.Reset()
	if err != nil {
		log.Crit("unable to reset store", "err", err)
		return
	}

	focusSrv, err := New(focusStore.Msgs())
	if err != nil {
		t.Fatalf("err: %s ", err)
	}

	wg := &sync.WaitGroup{}

	clients := make([]*client, numClients)

	run := func(idx int) {
		defer wg.Done()

		// BUG(mistone): OPEN / really should probably fail, though we'll test that it works today.
		vpName := "/"

		conn, conn2 := NewWSPair()

		c := &client{
			clname: fmt.Sprintf("%d", idx),
			name:   vpName,
			doc:    ot.NewDoc(),
			ws:     conn,
			hist:   []msg.Msg{},
		}
		c.st = ot.NewController(c, c)
		c.l = log.New(
			"obj", "cln",
			"client", log.Lazy{c.String},
		)
		clients[idx] = c

		_, err := focusSrv.Connect(conn2)
		if err != nil {
			panic(err)
		}

		conn.SetWriteTimeout(writeTimeout)
		err = conn.WriteJSON(msg.Msg{
			Cmd:  msg.C_OPEN,
			Name: vpName,
		})
		conn.CancelWriteTimeout()
		if err != nil {
			panic(err)
		}

		// read open resp
		m := msg.Msg{}
		conn.SetReadTimeout(readTimeout)
		err = conn.ReadJSON(&m)
		conn.CancelReadTimeout()
		if err != nil {
			panic(err)
		}

		c.loop()
	}

	wg.Add(numClients)
	for i := 0; i < numClients; i++ {
		go run(i)
	}
	wg.Wait()

	d := focusSrv.names["/"]
	sdrc := make(chan im.Readallresp)
	d <- im.Readall{Reply: sdrc}
	sdr := <-sdrc
	sd := sdr.Body

	log.Info("stat", "obj", "doc", "body", sd)

	for i := 0; i < numClients; i++ {
		st := clients[i].st
		log.Info("stat", "obj", "cln", "client", i, "body", clients[i].doc.String(), "clnst", st)
		if !st.IsSynchronized() {
			t.Fatalf("unsynchronized client[%d]; state: %q", i, st)
		}
	}

	for i := 0; i < numClients; i++ {
		s1 := clients[i].doc.String()
		if sd != s1 {
			t.Fatalf("error, doc[%d] != server doc\n\t%q\n\t%q\n\tstate: %q", i, s1, sd, clients[i].st)
		}
		for j := i + 1; j < numClients; j++ {
			s2 := clients[j].doc.String()
			if s1 != s2 {
				t.Fatalf("error, doc[%d] != doc[%d]\n\t%q\n\t%q\n\tstate1: %q\n\tstate2: %q", i, j, s1, s2, clients[i].st, clients[j].st)
			}
		}
	}
	t.Logf("iteration %d complete", iteration)
	log.Info("iteration complete", "iteration", iteration)
}

func TestRandom(t *testing.T) {
	iterations := 10
	flag.IntVar(&iterations, "iterations", 10, "number of iterations to run tests")

	for i := 0; i < iterations; i++ {
		testOnce(t, i)
	}
}
