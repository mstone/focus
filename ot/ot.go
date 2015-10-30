// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.
//
// Note: this file is a derivative work of
//
// https://github.com/Operational-Transformation/ot.py/blob/3777bee2c2cdb263d4ba09dd8ff0974b48f6b87c/ot/text_operation.py
// https://github.com/Operational-Transformation/ot.py/blob/3777bee2c2cdb263d4ba09dd8ff0974b48f6b87c/ot/client.py
// https://github.com/Operational-Transformation/ot.v/blob/d48f2598142236ee8980247060c98ba3175c464a/ListOperation.v
//
// Copyright © 2012-2013 Tim Baumann, http://timbaumann.info
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the “Software”), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package ot

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"unicode/utf8"

	"github.com/juju/errors"
)

func CloneRunes(body []rune) []rune {
	if len(body) == 0 {
		return nil
	}
	ret := make([]rune, len(body))
	copy(ret, body)
	return ret
}

func AsRunes(s string) []rune {
	rs := make([]rune, utf8.RuneCountInString(s))
	for i, r := range s {
		rs[i] = r
	}
	return rs
}

func AsString(rs []rune) string {
	buf := bytes.Buffer{}
	for _, r := range rs {
		buf.WriteRune(r)
	}
	return buf.String()
}

func AsRuneTree(s string) Tree {
	kids := make(Trees, utf8.RuneCountInString(s))
	for i, r := range s {
		kids[i] = Leaf(r)
	}
	return Branch(kids)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Apply(o Op, t *Tree) error {
	if !o.IsWith() || !t.IsBranch() {
		return errors.Errorf("Apply failed; o: %s, t: %s", o.String(), t.String())
	}

	tz := NewZipper(t, 0, 10)

	// olen := len(o.Kids)
	for _, o := range o.Kids {
		switch {
		case o.IsZero():
			continue
		case o.IsInsert():
			tz.Insert(o.Body.Clone())
			tz.Skip(1)
		case o.IsRetain():
			tz.Retain(o.Len())
		case o.IsDelete():
			tz.Delete(o.Len())
		case o.IsWith():
			err := Apply(o, tz.Current())
			if err != nil {
				return errors.Trace(err)
			}
		}
	}
	return nil
}

// shortenOp returns the prefix of o that compose1 will need to retain.
func shortenOp(o Op, nl int) (Op, error) {
	o = o.Clone()
	switch {
	case o.IsRetain():
		switch {
		case o.Size > nl:
			o.Size -= nl
		case o.Size == nl:
			return Z(), nil
		default:
			return Z(), errors.Errorf("shorten fail; nl > retain size; nl: %d, o: %s", nl, o.String())
		}
	case o.IsDelete():
		o.Size += nl
	case o.IsInsertLeaf():
		if nl != 1 {
			return Z(), errors.Errorf("shorten fail; tried to split atomic leaf")
		}
		return o, nil
	case o.IsInsertBranch():
		o.Body.Kids = o.Body.Kids[nl:]
	case o.IsWith():
		o.Kids = o.Kids[nl:] // BUG(mistone): how to shorten With ops?
	default:
		return Z(), errors.Errorf("shorten fail, unknown op: %s", o.String())
	}
	return o, nil
}

// shortenOps returns the suffixes of a and b that compose1 will need to recurse on.
func shortenOps(a Op, b Op) (Op, Op, error) {
	var a2, b2 Op
	var err error

	la := a.Len()
	lb := b.Len()

	switch {
	case la == lb:
		return Z(), Z(), nil
	case la > lb:
		a2, err = shortenOp(a, lb)
		return a2, Z(), errors.Trace(err)
	case la <= lb:
		b2, err = shortenOp(b, la)
		return Z(), b2, errors.Trace(err)
	}
	return Z(), Z(), errors.Errorf("ot.shortenOps() -- unreachable case, a: %s, b: %s", a, b)
}

func addDeleteOp(d Op, os Ops) Ops {
	if len(os) > 0 && os.First().IsInsert() {
		ret := Ops{}
		ret = append(ret, os.First().Clone())
		ret = append(ret, addDeleteOp(d, os.Rest())...)
		return ret
	} else {
		ret := Ops{}
		ret = append(ret, d)
		ret = append(ret, os...)
		return ret
	}
}

func Compose(as, bs Ops) (Ops, error) {
	cs, err := compose1(as, bs)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return Normalize(cs)
}

func compose1(as, bs Ops) (Ops, error) {
	ret := Ops{}
	rest := Ops{}
	hcs := Ops{}
	oc := Op{}
	var err error
	var sa, sb Op

	a := 0
	b := 0
	la := len(as)
	lb := len(bs)

	switch {
	case a == la && b == lb:
		break
	case la > 0 && as[a].IsZero():
		ret, err = compose1(as[a+1:], bs)
	case lb > 0 && bs[b].IsZero():
		ret, err = compose1(as, bs[b+1:])
	case a == la:
		ret = bs.Clone()
	case b == lb:
		ret = as.Clone()
	case la > 0 && as[a].IsDelete():
		// run insertions, then delete, then apply remaining effects
		rest, err = compose1(as[a+1:], bs)
		if err != nil {
			break
		}
		ret = addDeleteOp(as[a].Clone(), rest)
	case lb > 0 && bs[b].IsInsert():
		// as[a] is insert, retain, or empty so insert then apply remaining effects
		rest, err = compose1(as, bs[b+1:])
		if err != nil {
			break
		}
		ret = append(ret, bs[b].Clone())
		ret = append(ret, rest...)
	case la > 0 && lb > 0:
		// do as much as we can, then recurse in a new hypothetical world
		oa := as[a]
		ob := bs[b]

		sa, sb, err = shortenOps(oa, ob)
		if err != nil {
			break
		}

		has := Ops{}
		has = append(has, sa)
		has = append(has, as[a+1:]...)

		hbs := Ops{}
		hbs = append(hbs, sb)
		hbs = append(hbs, bs[b+1:]...)

		minlen := min(oa.Len(), ob.Len())
		switch {
		case oa.IsRetain() && ob.IsRetain():
			ret = append(ret, R(minlen))
		case oa.IsRetain() && ob.IsDelete():
			ret = append(ret, D(minlen))
		case oa.IsRetain() && ob.IsWith():
			ret = append(ret, ob.Clone())
		case oa.IsInsert() && ob.IsRetain():
			oc, err = shortenOp(oa, minlen)
			if err != nil {
				err = errors.Trace(err)
				break
			}
			ret = append(ret, oc)
		case oa.IsInsert() && ob.IsDelete():
			// insertion then deletion cancels
		case oa.IsInsert() && ob.IsWith():
			ta := oa.Body.Clone()
			err = Apply(ob, &ta)
			if err != nil {
				break
			}
			ret = append(ret, It(ta))
		case oa.IsWith() && ob.IsWith():
			kc := Ops{}
			kc, err = Compose(oa.Kids, ob.Kids)
			if err != nil {
				break
			}
			ret = append(ret, W(kc))
		case oa.IsWith() && ob.IsRetain():
			ret = append(ret, oa.Clone())
		case oa.IsWith() && ob.IsDelete():
			ret = append(ret, D(minlen))
		default:
			err = errors.Errorf("compose1 error: impossible case\n\tas: %s\n\tbs: %s", as.String(), bs.String())
		}
		if err != nil {
			break
		}

		hcs, err = compose1(has, hbs)
		if err != nil {
			break
		}

		ret = append(ret, hcs...)
	}
	return ret, errors.Trace(err)
}

func ComposeAll(all []Ops) (Ops, error) {
	ret := Ops{}
	var err error
	for _, os := range all {
		ret, err = Compose(ret, os)
		if err != nil {
			break
		}
	}
	return ret, errors.Trace(err)
}

func Transform(as, bs Ops) (Ops, Ops, error) {
	var r1, r2 Ops
	var err error

	if bs.Empty() {
		return as.Clone(), bs.Clone(), nil
	}

	r1, r2, err = transform1(as, bs)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	r1, err = Normalize(r1)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	r2, err = Normalize(r2)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	return r1, r2, nil
}

func transform1(as, bs Ops) (Ops, Ops, error) {
	a := 0
	b := 0

	var ra, rb, sa, sb Ops
	var ta, tb Op

	la := len(as)
	lb := len(bs)

	var err error

	switch {
	case a == la && b == lb:
		break
	case la > 0 && as.First().IsZero():
		sa, sb, err = transform1(as[a+1:], bs)
	case lb > 0 && bs.First().IsZero():
		sa, sb, err = transform1(as, bs[b+1:])
	case la > 0 && as.First().IsInsert():
		oa := as.First()
		ra.Insert(oa.Body)
		rb.Retain(oa.Len())
		sa, sb, err = transform1(as[a+1:], bs)
		if err != nil {
			break
		}
	case lb > 0 && bs.First().IsInsert():
		ob := bs.First()
		ra.Retain(ob.Len())
		rb.Insert(ob.Body)
		sa, sb, err = transform1(as, bs[b+1:])
		if err != nil {
			break
		}
	case la > 0 && lb > 0:
		oa := as.First()
		ob := bs.First()
		minlen := min(oa.Len(), ob.Len())
		ta, tb, err = shortenOps(*oa, *ob)
		if err != nil {
			break
		}

		has := Ops{}
		if !ta.IsZero() {
			has = append(has, ta)
		}
		has = append(has, as[a+1:]...)

		hbs := Ops{}
		if !tb.IsZero() {
			hbs = append(hbs, tb)
		}
		hbs = append(hbs, bs[b+1:]...)

		switch {
		case oa.IsRetain() && ob.IsRetain():
			ra.Retain(minlen)
			rb.Retain(minlen)
		case oa.IsWith() && ob.IsWith():
			var ka, kb Ops
			ka, kb, err = Transform(oa.Kids, ob.Kids)
			if err != nil {
				break
			}
			ra.With(ka)
			rb.With(kb)
		case oa.IsRetain() && ob.IsWith():
			ra.With(ob.Kids)
		case oa.IsWith() && ob.IsRetain():
			rb.With(oa.Kids)
		case oa.IsDelete() && ob.IsDelete():
		case oa.IsDelete() && ob.IsRetain():
		case oa.IsDelete() && ob.IsWith():
			ra.Delete(minlen)
		case oa.IsRetain() && ob.IsDelete():
		case oa.IsWith() && ob.IsDelete():
			rb.Delete(minlen)
		}
		sa, sb, err = transform1(has, hbs)
		if err != nil {
			break
		}
	default:
		err = errors.Errorf("transform failed, as: %s, bs: %s", as.String(), bs.String())
		if err != nil {
			break
		}
	}

	ret1 := append(ra, sa...)
	ret2 := append(rb, sb...)
	if err != nil {
		err = errors.Annotatef(err, "transform failed, as: %s, bs: %s", as.String(), bs.String())
	}
	return ret1, ret2, errors.Trace(err)
}

func Normalize(os Ops) (Ops, error) {
	swap := func(a, b *Op) {
		*a, *b = *b, *a
	}

	ret := os.Clone()

	for i := 0; i < len(ret)-1; i++ {
		if ret[i].IsDelete() && ret[i+1].IsInsert() {
			swap(&ret[i], &ret[i+1])
		}
	}

	ret2 := Ops{}
	for _, o := range ret {
		switch {
		case o.IsZero():
			continue
		case o.IsInsert():
			ret2.Insert(o.Body)
		case o.IsDelete():
			ret2.Delete(o.Size)
		case o.IsRetain():
			ret2.Retain(o.Size)
		case o.IsWith():
			ret2.With(o.Kids)
		default:
			return nil, errors.Errorf("normalize got bad op: %s", o.String())
		}
	}

	return ret2, nil
}

type Doc struct {
	mu sync.Mutex
	// Current text
	body Tree
}

func NewDoc() *Doc {
	d := new(Doc)
	d.body = Branch(nil)
	return d
	// return &Doc{
	// 	mu:   sync.Mutex{},
	// 	body: make([]rune{},
	// }
}

func (d *Doc) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	return len(d.body.Kids)
}

func (d *Doc) String() string {
	d.mu.Lock()
	defer d.mu.Unlock()

	// buf := bytes.Buffer{}
	// for _, r := range d.body {
	// 	buf.WriteRune(r)
	// }
	// return buf.String()
	return d.body.String()
}

func (d *Doc) Apply(os Ops) {
	d.mu.Lock()
	defer d.mu.Unlock()

	body := d.body.Clone()
	err := Apply(W(os), &body)
	if err != nil {
		panic(err)
	}
	d.body = body
}

func RandIntn(n int) int {
	b, _ := rand.Int(rand.Reader, big.NewInt(int64(n)))
	return int(b.Int64())
}

func (d *Doc) GetRandomOps(numChars int) Ops {
	ops := Ops{}
	size := d.Len()
	op := 0
	if size > 0 {
		op = RandIntn(2)
	}
	switch op {
	case 0: // insert
		s := fmt.Sprintf("%x", RandIntn(numChars*8))
		pos := 0
		if size > 0 {
			pos = RandIntn(size)
		}
		ops = NewInsert(size, pos, s)
	case 1: // delete
		if size == 1 {
			ops = NewDelete(1, 0, 1)
		} else {
			d := RandIntn(size)
			pos := 0
			if size-d > 0 {
				pos = RandIntn(size - d)
			}
			ops = NewDelete(size, pos, d)
		}
	}

	return ops.Clone()
}

// C concatenates multiple Ops slices
func C(os ...Ops) Ops {
	ret := Ops{}
	for _, o := range os {
		ret = append(ret, o...)
	}
	return ret
}

// Is converts s into a slice of rune leaf insertion ops
func Is(s string) Ops {
	if len(s) == 0 {
		return nil
	}
	os := make(Ops, utf8.RuneCountInString(s))
	for i, r := range s {
		os[i] = Op{Tag: O_INSERT, Body: Leaf(r)}
	}
	return os
}

// Ic converts r into a single leaf insertion op
func Ic(r rune) Op {
	return Op{Tag: O_INSERT, Body: Leaf(r)}
}

// Ir converts rs into a slice of rune leaf insertion ops
func Ir(rs []rune) Ops {
	if len(rs) == 0 {
		return nil
	}
	os := make(Ops, len(rs))
	for i, r := range rs {
		os[i] = Op{Tag: O_INSERT, Body: Leaf(r)}
	}
	return os
}

// It creates an insertion op based on t
func It(t Tree) Op {
	if t.Len() == 0 {
		return Z()
	}
	return Op{Tag: O_INSERT, Body: t}
}

// R creates a retain op
func R(n int) Op {
	if n == 0 {
		return Z()
	}
	return Op{Tag: O_RETAIN, Size: n}
}

// Rs creates an op slice containing a single retain op
func Rs(n int) Ops {
	return Ops{R(n)}
}

// D creates a delete op
func D(n int) Op {
	if n == 0 {
		return Z()
	}
	if n < 0 {
		n = -n
	}
	return Op{Tag: O_DELETE, Size: -n}
}

// Ds creates an op slice containing a single delete op
func Ds(n int) Ops {
	return Ops{D(n)}
}

// W returns a With op wrapping kids
func W(kids Ops) Op {
	return Op{Tag: O_WITH, Kids: kids}
}

// Ws returns an op slice containing a single With op wrapping kids
func Ws(kids Ops) Ops {
	return Ops{W(kids)}
}

// Z returns a nil op
func Z() Op {
	return Op{Tag: O_NIL}
}

// Zs returns an op slice wrapping a single nil op
func Zs() Ops {
	return Ops{Z()}
}

func NewInsert(docLen int, pos int, s string) Ops {
	if pos < 0 || pos > docLen+1 {
		panic(errors.Errorf("bad position; insert is out of range; pos: %d, s: %q", pos, s))
	}

	is := Is(s)
	os := make(Ops, len(is)+2)
	os[0] = R(pos)
	copy(os[1:], is)
	os[len(os)-1] = R(docLen - pos)
	return os
}

func NewDelete(docLen int, pos int, length int) Ops {
	if pos < 0 || pos+length > docLen+1 {
		panic(errors.Errorf("bad position; delete is out of range: pos: %d, len: %d", pos, length))
	}

	return Ops{R(pos), D(length), R(docLen - length - pos)}
}

type State int

const (
	CS_SYNCED State = iota
	CS_WAIT_ONE
	CS_WAIT_MANY
)

type Sender interface {
	Send(rev int, hash string, ops Ops)
}

type Receiver interface {
	Recv(ops Ops)
}

type Controller struct {
	state     State
	conn      Sender
	client    Receiver
	first     Ops
	rest      []Ops
	serverRev int
	serverDoc *Doc
}

func (c *Controller) String() string {
	return fmt.Sprintf("St[%d, %s, %s, %d, %s]", c.state, c.first, c.rest, c.serverRev, c.serverDoc.String())
}

func NewController(sender Sender, receiver Receiver) *Controller {
	return &Controller{
		state:     CS_SYNCED,
		conn:      sender,
		client:    receiver,
		first:     nil,
		rest:      nil,
		serverRev: 0,
		serverDoc: NewDoc(),
	}
}

func (c *Controller) OnClientWrite(ops Ops) {
	ops, err := Normalize(ops.Clone())
	if err != nil {
		panic(err)
	}
	switch c.state {
	case CS_SYNCED:
		c.first = ops
		c.conn.Send(c.serverRev, c.serverDoc.String(), ops)
		c.state = CS_WAIT_ONE
	case CS_WAIT_ONE:
		c.rest = []Ops{ops}
		c.state = CS_WAIT_MANY
	case CS_WAIT_MANY:
		c.rest = append(c.rest, ops)
	}
}

func (c *Controller) OnServerAck(rev int, ops Ops) {
	switch c.state {
	case CS_SYNCED:
		panic("bad ack")
	case CS_WAIT_ONE:
		c.serverRev = rev
		c.serverDoc.Apply(ops)
		c.first = nil
		c.state = CS_SYNCED
	case CS_WAIT_MANY:
		c.serverRev = rev
		c.serverDoc.Apply(ops)
		cs, err := ComposeAll(c.rest)
		if err != nil {
			panic("bad ack, compose failed")
		}
		c.first, err = Normalize(cs)
		if err != nil {
			panic("bad ack, normalize failed")
		}
		c.rest = nil
		c.conn.Send(c.serverRev, c.serverDoc.String(), c.first)
		c.state = CS_WAIT_ONE
	}
}

func (c *Controller) OnServerWrite(rev int, ops Ops) {
	c.serverDoc.Apply(ops)
	switch c.state {
	case CS_SYNCED:
		c.serverRev = rev
		c.client.Recv(ops)
	case CS_WAIT_ONE:
		c.serverRev = rev
		first2, ops2, err := Transform(c.first, ops)
		if err != nil {
			panic("bad write, transform failed in CS_WAIT_ONE")
		}
		c.first = first2
		c.client.Recv(ops2)
	case CS_WAIT_MANY:
		c.serverRev = rev
		first2, ops2, err := Transform(c.first, ops)
		if err != nil {
			panic("bad write, transform failed in CS_WAIT_MANY, pt 1")
		}
		cs, err := ComposeAll(c.rest)
		if err != nil {
			panic("bad write, compose failed")
		}
		rest2, ops3, err := Transform(cs, ops2)
		if err != nil {
			panic("bad write, transform failed in CS_WAIT_MANY, pt 2")
		}
		c.first = first2
		c.rest = []Ops{rest2}
		c.client.Recv(ops3)
	}
}

func (c *Controller) IsSynchronized() bool {
	return c.state == CS_SYNCED
}

func (c *Controller) ServerRev() int {
	return c.serverRev
}
