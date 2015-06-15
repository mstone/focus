package server

import (
	im "github.com/mstone/focus/internal/msgs"
	"github.com/mstone/focus/msg"
	"github.com/mstone/focus/ot"
	"reflect"
	"testing"
)

type cl struct {
	t        *testing.T
	wsa, wsb *ws
	doc      *ot.Doc
	st       *ot.Controller
	num      int
}

func (c *cl) Send(rev int, hash string, ops ot.Ops) {
	c.t.Logf("S: %d, rev: %d, ops: %s", c.num, rev, ops)
	m := msg.Msg{
		Cmd:  msg.C_WRITE,
		Fd:   0,
		Rev:  rev,
		Hash: hash,
		Ops:  ops,
	}
	c.wsa.WriteJSON(m)
}

func (c *cl) Recv(ops ot.Ops) {
	// c.t.Logf("W: %d, rev: %d, ops: %s, prev: %s", c.num, rev, ops, c.doc.String())
	c.doc.Apply(ops)
}

func TestStatic(t *testing.T) {
	srv, _ := New(nil)

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
			cls[i].st.OnServerAck(m.Rev, m.Ops)
		case msg.C_WRITE:
			cls[i].st.OnServerWrite(m.Rev, m.Ops)
		}
	}
	recvFlight := func() {
		for i := 0; i < 4; i++ {
			recv(i)
		}
	}
	I := ot.I
	D := ot.D
	R := ot.R
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

func TestStaticCommute(t *testing.T) {
	//     h
	//  -------    |
	// *-a-*-b-*   |
	// |c  |d  |e  j
	// *-f-*-g-*   |
	//  -------    |
	//     i
	I := ot.I
	D := ot.D
	R := ot.R

	a := ot.Ops{R(1), D(1), R(1), I("f"), R(1)}
	b := ot.Ops{I("1"), D(2), R(2)}
	c := ot.Ops{R(3), I("6"), R(1)}
	d := ot.Ops{R(2), I("6"), R(2)}
	e := ot.Ops{R(1), I("6"), R(2)}
	f := ot.Ops{R(1), D(1), R(2), I("f"), R(1)}
	g := ot.Ops{I("1"), D(2), R(3)}
	h := ot.Ops{I("1f"), D(3), R(1)}
	i := ot.Ops{I("1f"), D(3), R(2)}
	j := ot.Ops{R(2), I("6"), R(1)}
	_ = a
	_ = b
	_ = c
	_ = d
	_ = e
	_ = f
	_ = g
	_ = h
	_ = i
	_ = j

	CEQ := func(l0 string, lhs ot.Ops, l1 string, r1 ot.Ops, l2 string, r2 ot.Ops) {
		rhs := ot.Compose(r1, r2)
		if !reflect.DeepEqual(lhs, rhs) {
			t.Errorf("fail: %s: %s != %s <- C(%s: %s, %s: %s)", l0, lhs, rhs, l1, r1, l2, r2)
		} else {
			t.Logf("%s = C(%s, %s) good", l0, l1, l2)
		}
	}
	TEQ := func(l0 string, srv2 ot.Ops, l1 string, cln2 ot.Ops, l2 string, cln ot.Ops, l3 string, srv ot.Ops) {
		fsrv, fcln := ot.Transform(cln, srv)
		t.Logf("T(%s: %s, %s: %s) -> %s, %s", l2, cln, l3, srv, fsrv, fcln)
		if !reflect.DeepEqual(srv2, fsrv) {
			t.Errorf("fail: srv2 wrong: %s: %s != %s <--- result", l0, srv2, fsrv)
		}
		if !reflect.DeepEqual(cln2, fcln) {
			t.Errorf("fail: cln2 wrong: %s: %s != %s <--- result", l1, cln2, fcln)
		}
		x := ot.Compose(srv, fsrv)
		y := ot.Compose(cln, fcln)
		if !reflect.DeepEqual(x, y) {
			t.Errorf("fail: T(%s, %s) did not commute; got cln'.srv = %s != %s = srv'.cln", l2, l3, y, x)
		} else {
			t.Logf("T(%s, %s) commutes", l2, l3)
		}
	}

	ba := ot.Compose(a, b)
	gf := ot.Compose(f, g)
	_ = ba
	_ = gf

	CEQ("h", h, "a", a, "b", b)
	// CEQ("i", i, "f", f, "g", g) // uhoh!

	TEQ("d", d, "f", f, "c", c, "a", a)
	TEQ("e", e, "g", g, "d", d, "b", b)

	// TEQ("e", e, "g.f", gf, "c", c, "b.a", ba) // waaah!
	// TEQ("j", j, "g.f", gf, "c", c, "b.a", ba) // waaah!
	// TEQ("j", j, "g", g, "d", d, "b", b) // yikes!
	TEQ("j", j, "i", i, "c", c, "h", h)
}
