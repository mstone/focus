package server

import (
	im "github.com/mstone/focus/internal/msgs"
	"github.com/mstone/focus/msg"
	"github.com/mstone/focus/ot"
	"testing"
)

type cl struct {
	t        *testing.T
	wsa, wsb *ws
	doc      *ot.Doc
	st       *ot.Controller
	num      int
}

func (c *cl) Send(rev int, ops ot.Ops) {
	c.t.Logf("S: %d, rev: %d, ops: %s", c.num, rev, ops)
	m := msg.Msg{
		Cmd: msg.C_WRITE,
		Fd:  0,
		Rev: rev,
		Ops: ops,
	}
	c.wsa.WriteJSON(m)
}

func (c *cl) Recv(ops ot.Ops) {
	// c.t.Logf("W: %d, rev: %d, ops: %s, prev: %s", c.num, rev, ops, c.doc.String())
	c.doc.Apply(ops)
}

func TestStatic(t *testing.T) {
	srv, _ := New()

	cls := [4]cl{}

	for i := 0; i < 4; i++ {
		cls[i].t = t
		cls[i].num = i
		cls[i].st = ot.NewController(&cls[i], &cls[i])
		cls[i].doc = ot.NewDoc()
		cls[i].wsa, cls[i].wsb = NewWSPair()

		srv.Connect(cls[i].wsb)

		cls[i].wsa.WriteJSON(msg.Msg{
			Cmd:  msg.C_OPEN,
			Name: "/",
		})

		m := msg.Msg{}
		cls[i].wsa.ReadJSON(&m)
		cls[i].wsa.ReadJSON(&m)
	}

	send := func(i int, ops ot.Ops) {
		cls[i].doc.Apply(ops)
		cls[i].st.OnClientWrite(ops)
	}
	recv := func(i int) {
		m := msg.Msg{}
		cls[i].wsa.ReadJSON(&m)
		switch m.Cmd {
		case msg.C_WRITE_RESP:
			cls[i].st.OnServerAck(m.Rev)
		case msg.C_WRITE:
			cls[i].st.OnServerWrite(m.Rev, m.Ops)
		}
	}
	recvFlight := func() {
		for i := 0; i < 4; i++ {
			recv(i)
		}
	}
	I := func(s string) ot.Op {
		return ot.Op{0, ot.AsRunes(s)}
	}
	D := func(n int) ot.Op {
		return ot.Op{-n, nil}
	}
	R := func(n int) ot.Op {
		return ot.Op{n, nil}
	}
	_ = recvFlight
	_ = I
	_ = D
	_ = R

	send(0, ot.Ops{R(0), I("4"), R(0)})
	send(0, ot.Ops{R(0), I("0"), R(1)})
	send(0, ot.Ops{R(1), R(0), R(1)})
	send(1, ot.Ops{R(0), I("6"), R(0)})
	send(1, ot.Ops{R(0), D(1), R(0)})
	send(1, ot.Ops{R(0), I("6"), R(0)})
	send(2, ot.Ops{R(0), I("2"), R(0)})
	send(2, ot.Ops{R(0), D(1), R(0)})
	send(2, ot.Ops{R(0), I("0"), R(0)})
	send(3, ot.Ops{R(0), I("2"), R(0)})
	send(3, ot.Ops{R(0), D(1), R(0)})
	send(3, ot.Ops{R(0), I("6"), R(0)})
	// for i := 0; i < 8; i++ {
	// 	recvFlight()
	// }
	recv(0)
	recv(0)
	recv(1)
	recv(2)
	recv(3)
	recv(1)
	recv(1)
	recv(2)
	recv(3)
	recv(0)
	recv(2)
	recv(3)
	recv(3)
	recv(0)
	recv(1)
	recv(1)
	recv(2)
	recv(2)
	recv(2)
	recv(3)
	recv(0)
	recv(3)
	recv(0)
	recv(0)
	recv(1)
	recv(1)
	recv(2)
	recv(3)
	recv(3)
	recv(0)
	recv(1)
	recv(2)

	d := srv.names["/"]
	sdrc := make(chan im.Readallresp)
	d <- im.Readall{sdrc}
	sdr := <-sdrc
	sd := sdr.Body
	t.Logf("doc[s] = %q", sd)
	for i := 0; i < 4; i++ {
		s1 := cls[i].doc.String()
		t.Logf("doc[%d] = %q", i, s1)
		if sd != s1 {
			t.Errorf("error, doc[%d] != server doc\n\t%q\n\t%q", i, s1, sd)
		}
		for j := i + 1; j < numClients; j++ {
			s2 := cls[i].doc.String()
			if s1 != s2 {
				t.Errorf("error, doc[%d] != doc[%d]\n\t%q\n\t%q", i, j, s1, s2)
			}
		}
	}
}
