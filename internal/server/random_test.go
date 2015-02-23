// Copyright 2015 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package server

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	log "gopkg.in/inconshreveable/log15.v2"
	"math/big"
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

const numClients = 10
const numRounds = 1
const numChars = 8

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
	mu      sync.Mutex
	wg      *sync.WaitGroup
	clname  string
	name    string
	fd      int
	ws      connection.WebSocket
	rev     int
	doc     *ot.Doc
	st      ot.State
	numSend int
	numRecv int
	l       log.Logger
}

func randIntn(n int) int {
	b, _ := rand.Int(rand.Reader, big.NewInt(int64(n)))
	return int(b.Int64())
}

func (c *client) sendRandomOps() {
	c.mu.Lock()
	defer c.mu.Unlock()

	ops := ot.Ops{}
	size := c.doc.Len()
	op := 0
	if size > 0 {
		op = randIntn(2)
	}
	switch op {
	case 0: // insert
		s := fmt.Sprintf("%x", randIntn(numChars))
		pos := 0
		if size > 0 {
			pos = randIntn(size)
		}
		ops = ot.NewInsert(size, pos, s)
	case 1: // delete
		if size == 1 {
			ops = ot.NewDelete(1, 0, 1)
		} else {
			d := randIntn(size)
			pos := 0
			if size-d > 0 {
				pos = randIntn(size - d)
			}
			ops = ot.NewDelete(size, pos, d)
		}
	}

	c.l.Info("genn", "ops", ops, "docsize", size, "doc", c.doc.String(), "docp", fmt.Sprintf("%p", c.doc))
	c.doc.Apply(ops.Clone())
	c.st = c.st.Client(c, ops.Clone())
}

func (c *client) Send(ops ot.Ops) {
	c.ws.SetWriteTimeout(1000 * time.Millisecond)
	m := msg.Msg{
		Cmd: msg.C_WRITE,
		Fd:  c.fd,
		Rev: c.rev,
		Ops: ops.Clone(),
	}
	c.l.Info("send", "num", c.numSend, "rev", c.rev, "ops", ops)
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
	// c.l.Info("recv", "num", c.numRecv, "kind", "wrt", "rev", rev, "ops", ops, "clnrev", c.rev, "clnhist", c.doc.String())
	c.doc.Apply(ops.Clone())
	c.rev = rev
}

func (c *client) Ack(rev int) {
	// c.l.Info("recv", "num", c.numRecv, "kind", "ack", "rev", rev, "clnrev", c.rev, "clnhist", c.doc.String())
	c.rev = rev
}

func (c *client) onWriteResp(m msg.Msg) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.st = c.st.Ack(c, m.Rev)
}

func (c *client) onWrite(m msg.Msg) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.st = c.st.Server(c, m.Rev, m.Ops.Clone())
}

func (c *client) writeLoop() {
	defer c.wg.Done()

	for i := 0; i < numRounds; i++ {
		c.sendRandomOps()
	}
}

func (c *client) readLoop() {
	defer c.wg.Done()

Loop:
	for {
		m := msg.Msg{}
		c.ws.SetReadTimeout(500 * time.Millisecond)
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
		var err error
		defer wg.Done()

		// BUG(mistone): OPEN / really should probably fail, though we'll test that it works today.
		vpName := "/"

		cwg := &sync.WaitGroup{}

		conn, conn2 := NewWSPair()

		c := &client{
			mu:     sync.Mutex{},
			wg:     cwg,
			clname: fmt.Sprintf("%d", idx),
			name:   vpName,
			rev:    0,
			doc:    ot.NewDoc(),
			st:     &ot.Synchronized{},
			ws:     conn,
		}
		c.l = log.New(
			"obj", "cln",
			"client", log.Lazy{c.String},
			"#", log.Lazy{func() int {
				return c.numRecv + c.numSend
			}},
		)
		clients[idx] = c

		focusSrv.Connect(conn2)

		conn.SetWriteTimeout(1000 * time.Millisecond)
		err = conn.WriteJSON(msg.Msg{
			Cmd:  msg.C_OPEN,
			Name: vpName,
		})
		conn.CancelWriteTimeout()
		if err != nil {
			t.Errorf("unable to write OPEN, err: %q", err)
		}

		// read open resp
		m := msg.Msg{}
		conn.SetReadTimeout(1000 * time.Millisecond)
		err = conn.ReadJSON(&m)
		conn.CancelReadTimeout()
		if err != nil {
			t.Errorf("server unable to read OPEN_RESP, err: %q", err)
		}
		conn.CancelReadTimeout()

		if m.Cmd != msg.C_OPEN_RESP {
			t.Errorf("client did not get an OPEN_RESP; msg: %+v", m)
		}

		if m.Name != vpName {
			t.Errorf("client got OPEN_RESP with wrong vaporpad: %s vs %+v", vpName, m)
		}
		c.name = vpName
		c.fd = m.Fd

		// read open resp
		m = msg.Msg{}
		conn.SetReadTimeout(1000 * time.Millisecond)
		err = conn.ReadJSON(&m)
		conn.CancelReadTimeout()
		if err != nil {
			t.Errorf("server unable to read first WRITE, err: %q", err)
		}
		conn.CancelReadTimeout()

		if m.Cmd != msg.C_WRITE {
			t.Errorf("client did not get first WRITE; msg: %+v", m)
		}

		if m.Fd != c.fd {
			t.Errorf("client got first WRITE with wrong fd: %s vs %+v", c.fd, m)
		}
		c.onWrite(m)

		cwg.Add(2)
		go c.writeLoop()
		go c.readLoop()
		cwg.Wait()
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
	for i := 0; i < numClients; i++ {
		s1 := clients[i].doc.String()
		if sd != s1 {
			t.Fatalf("error, doc[%d] != server doc\n\t%q\n\t%q", i, s1, sd)
		}
		for j := i + 1; j < numClients; j++ {
			s2 := clients[j].doc.String()
			if s1 != s2 {
				t.Fatalf("error, doc[%d] != doc[%d]\n\t%q\n\t%q", i, j, s1, s2)
			}
		}
	}
}

func TestRandom(t *testing.T) {
	for i := 0; i < 90; i++ {
		testOnce(t)
	}
}
