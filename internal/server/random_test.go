// Copyright 2015 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package server

import (
	"encoding/json"
	"fmt"
	log "gopkg.in/inconshreveable/log15.v2"
	"sync"
	"testing"
	"time"

	"github.com/mstone/focus/internal/connection"
	im "github.com/mstone/focus/internal/msgs"
	"github.com/mstone/focus/msg"
	"github.com/mstone/focus/ot"
)

// const numClients = 4
// const numRounds = 3
// const numChars = 8

const numClients = 3
const numRounds = 3
const numChars = 3
const readTimeout = 50 * time.Millisecond
const writeTimeout = 500 * time.Millisecond

type ws struct {
	rq, wq chan interface{}
	rt, wt *time.Timer
}

func NewWSPair() (*ws, *ws) {
	q1 := make(chan interface{}, numClients*numRounds*2)
	q2 := make(chan interface{}, numClients*numRounds*2)
	w1 := &ws{
		rq: q1,
		wq: q2,
		rt: time.NewTimer(time.Duration(0)),
		wt: time.NewTimer(time.Duration(0)),
	}
	w2 := &ws{
		rq: q2,
		wq: q1,
		rt: time.NewTimer(time.Duration(0)),
		wt: time.NewTimer(time.Duration(0)),
	}
	w1.rt.Stop()
	w1.wt.Stop()
	w2.rt.Stop()
	w2.wt.Stop()
	return w1, w2
}

func (w *ws) ReadJSON(v interface{}) error {
	select {
	case <-w.rt.C:
		return fmt.Errorf("ws read timeout")
	case v2 := <-w.rq:
		js, _ := json.Marshal(v2)
		return json.Unmarshal(js, v)
	}
	return nil
}

func (w *ws) WriteJSON(v interface{}) error {
	select {
	case <-w.wt.C:
		return fmt.Errorf("ws write timeout")
	case w.wq <- v:
		return nil
	}
	return nil
}

func (w *ws) SetReadTimeout(d time.Duration) error {
	w.rt.Reset(d)
	return nil
}

func (w *ws) SetWriteTimeout(d time.Duration) error {
	w.wt.Reset(d)
	return nil
}

func (w *ws) CancelReadTimeout() error {
	w.rt.Stop()
	return nil
}

func (w *ws) CancelWriteTimeout() error {
	w.wt.Stop()
	return nil
}

type client struct {
	clname  string
	name    string
	ws      connection.WebSocket
	rev     int
	doc     *ot.Doc
	st      ot.State
	numSend int
	numRecv int
	l       log.Logger
	hist    []msg.Msg
}

func (c *client) sendRandomOps() {
	size := c.doc.Len()
	ops := c.doc.GetRandomOps(numChars)

	c.doc.Apply(ops)
	c.st = c.st.Client(c, ops.Clone())
	c.l.Info("genn", "ops", ops, "docsize", size, "doc", c.doc.String(), "docp", fmt.Sprintf("%p", c.doc), "clnhist", c.doc.String(), "clnst", c.st)
}

func (c *client) Send(ops ot.Ops) {
	c.ws.SetWriteTimeout(writeTimeout)
	m := msg.Msg{
		Cmd: msg.C_WRITE,
		Rev: c.rev,
		Ops: ops.Clone(),
	}
	// c.l.Info("send", "num", c.numSend, "rev", c.rev, "ops", ops)
	err := c.ws.WriteJSON(m)
	c.ws.CancelWriteTimeout()
	if err != nil {
		panic("client unable to send WRITE: " + err.Error())
	}
	c.numSend++
}

func (c *client) String() string {
	return fmt.Sprintf("%s", c.clname)
}

func (c *client) Recv(rev int, ops ot.Ops) {
	c.doc.Apply(ops.Clone())
	c.rev = rev
	c.l.Info("stat", "body", c.doc.String(), "clnst", c.st)
}

func (c *client) Ack(rev int) {
	c.l.Info("recv", "num", c.numRecv, "kind", "ack", "rev", rev, "clnrev", c.rev, "clnhist", c.doc.String(), "clnst", c.st)
	c.rev = rev
}

func (c *client) onWriteResp(m msg.Msg) {
	c.st = c.st.Ack(c, m.Rev)
}

func (c *client) onWrite(m msg.Msg) {
	c.l.Info("recv", "num", c.numRecv, "kind", "wrt", "rev", m.Rev, "ops", m.Ops, "clnrev", c.rev, "clnhist", c.doc.String(), "clnst", c.st)
	c.st = c.st.Server(c, m.Rev, m.Ops.Clone())
}

func (c *client) loop() {
	round := 0

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

func testOnce(t *testing.T) {
	log.Crit("boot")
	focusSrv, err := New()
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
			rev:    0,
			doc:    ot.NewDoc(),
			st:     &ot.Synchronized{},
			ws:     conn,
			hist:   []msg.Msg{},
		}
		c.l = log.New(
			"obj", "cln",
			"client", log.Lazy{c.String},
		)
		clients[idx] = c

		focusSrv.Connect(conn2)

		conn.WriteJSON(msg.Msg{
			Cmd:  msg.C_OPEN,
			Name: vpName,
		})

		// read open resp
		m := msg.Msg{}
		conn.ReadJSON(&m)

		c.loop()
	}

	wg.Add(numClients)
	for i := 0; i < numClients; i++ {
		go run(i)
	}
	wg.Wait()

	d := focusSrv.names["/"]
	sdrc := make(chan im.Readallresp)
	d <- im.Readall{sdrc}
	sdr := <-sdrc
	sd := sdr.Body

	log.Info("stat", "obj", "doc", "body", sd)

	for i := 0; i < numClients; i++ {
		st := clients[i].st
		log.Info("stat", "obj", "cln", "client", i, "body", clients[i].doc.String(), "clnst", st)
		if !ot.IsSynchronized(st) {
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
}

func TestRandom(t *testing.T) {
	for i := 0; i < 300; i++ {
		testOnce(t)
	}
}
