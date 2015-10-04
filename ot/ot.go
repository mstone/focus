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
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"unicode/utf8"
)

type Tag int

const (
	O_NIL Tag = iota
	O_INSERT
	O_RETAIN
	O_DELETE
	O_WITH
)

type Op struct {
	// Tag indicates what kind of Op we have
	Tag Tag

	// Len is either delete-len (if negative) or retain-len (if positive)
	Size int

	// Body is the text to be inserted
	Body []rune

	// Kids are the child-ops for parent With operations
	Kids Ops
}

func CloneRunes(body []rune) []rune {
	if len(body) == 0 {
		return nil
	}
	ret := make([]rune, len(body))
	copy(ret, body)
	return ret
}

func (o Op) Clone() Op {
	return Op{
		Tag:  o.Tag,
		Size: o.Size,
		Body: CloneRunes(o.Body),
		Kids: o.Kids.Clone(),
	}
}

func (o *Op) IsRetain() bool {
	if o == nil {
		return false
	}
	return o.Tag == O_RETAIN && o.Size > 0
}

func (o *Op) IsDelete() bool {
	if o == nil {
		return false
	}
	return o.Tag == O_DELETE && o.Size < 0
}

func (o *Op) IsInsert() bool {
	if o == nil {
		return false
	}
	return o.Tag == O_INSERT && o.Size == 0 && len(o.Body) > 0
}

func (o *Op) IsWith() bool {
	if o == nil {
		return false
	}
	return o.Tag == O_WITH
}

func (o *Op) IsZero() bool {
	if o == nil {
		return true
	}
	return o.Size == 0 && len(o.Body) == 0
}

func (o *Op) Len() int {
	switch {
	case o.IsZero():
		return 0
	case o.IsDelete():
		return -o.Size
	case o.IsRetain():
		return o.Size
	case o.IsInsert():
		return len(o.Body)
	case o.IsWith():
		return len(o.Kids) // BUG(mistone): what should Len() return for With ops?
	default:
		panic(fmt.Sprintf("len got bad op, %s", o.String()))
	}
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

func (o *Op) String() string {
	switch {
	case o == nil:
		return "N"
	case o.IsDelete():
		return fmt.Sprintf("D%d", -o.Size)
	case o.IsRetain():
		return fmt.Sprintf("R%d", o.Size)
	case o.IsInsert():
		return fmt.Sprintf("I%s", AsString(o.Body))
	case o.IsZero():
		return "Z"
	case o.IsWith():
		return fmt.Sprintf("W%s", o.Kids)
	default:
		return fmt.Sprintf("E%#v", o)
	}
}

type Ops []Op

func (os Ops) Clone() Ops {
	if len(os) == 0 {
		return nil
	}
	os2 := make(Ops, len(os))
	for i, op := range os {
		os2[i] = op.Clone()
	}
	return os2
}

func (os Ops) String() string {
	if len(os) > 0 {
		strs := []string{}
		for _, o := range os {
			strs = append(strs, o.String())
		}
		return fmt.Sprintf("[%s]", strings.Join(strs, " "))
	} else {
		return "[]"
	}
}

func (os Ops) First() *Op {
	return &os[0]
}

func (os Ops) Last() *Op {
	return &os[len(os)-1]
}

func (os Ops) Rest() Ops {
	return os[1:]
}

func (os Ops) Empty() bool {
	return len(os) == 0
}

func (op *Op) extendBody(rhs []rune) {
	lhs := op.Body
	op.Body = make([]rune, len(lhs)+len(rhs))
	copy(op.Body, lhs)
	copy(op.Body[len(lhs):], rhs)
}

func (os *Ops) insertPenultimate(op Op) {
	rhs := *os
	rlen := len(rhs)
	lhs := make(Ops, rlen+1)
	copy(lhs, rhs[:rlen-1])
	lhs[rlen-1] = op
	lhs[rlen] = rhs[rlen-1]
	*os = lhs
}

func (os *Ops) insertUltimate(op Op) {
	rhs := *os
	lhs := make(Ops, len(rhs))
	copy(lhs, rhs)
	lhs = append(lhs, op)
	*os = lhs
}

func (os *Ops) Insert(body []rune) {
	ops := *os
	olen := len(ops)
	switch {
	case len(body) == 0:
		break
	case olen > 0 && os.Last().IsInsert():
		ops.Last().extendBody(body)
	case olen > 0 && os.Last().IsDelete():
		if olen > 1 && ops[olen-2].IsInsert() {
			(&ops[olen-2]).extendBody(body)
		} else {
			os.insertPenultimate(Ir(body))
		}
	default:
		os.insertUltimate(Ir(body))
	}
}

func (os *Ops) Retain(size int) {
	switch {
	case size == 0:
		return
	case len(*os) > 0 && os.Last().IsRetain():
		os.Last().Size += size
	default:
		os.insertUltimate(R(size))
	}
}

func (os *Ops) Delete(size int) {
	ops := *os
	olen := len(ops)
	if size == 0 {
		return
	}
	if size > 0 {
		size = -size
	}
	if olen > 0 && ops[olen-1].IsDelete() {
		ops[olen-1].Size += size
	} else {
		os.insertUltimate(D(size))
	}
}

func (os *Ops) With(kids Ops) {
	os.insertUltimate(W(kids)) // BUG(mistone): should With() fold into previous With ops?
}

func (o Op) MarshalJSON() ([]byte, error) {
	switch {
	case o.IsDelete() || o.IsRetain():
		return json.Marshal(o.Size)
	case o.IsInsert():
		return json.Marshal(AsString(o.Body))
	case o.IsWith():
		return json.Marshal(o.Kids)
	default:
		return nil, fmt.Errorf("error: MarshalJSON() got bad op, o: %s", o.String())
	}
}

func (o *Op) UnmarshalJSON(data []byte) error {
	switch {
	case len(data) == 0:
		return fmt.Errorf("illegal op: %q", data)
	case data[0] == '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		o.Body = AsRunes(s)
		o.Tag = O_INSERT
		return nil
	case data[0] == '[':
		o.Tag = O_WITH
		return json.Unmarshal(data, &o.Kids)
	case data[0] == '-':
		o.Tag = O_DELETE
		return json.Unmarshal(data, &o.Size)
	default:
		o.Tag = O_RETAIN
		return json.Unmarshal(data, &o.Size)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func shorten(o Op, nl int) (Op, error) {
	o = o.Clone()
	switch {
	case o.IsRetain():
		o.Size -= nl
	case o.IsDelete():
		o.Size += nl
	case o.IsInsert():
		o.Body = o.Body[nl:]
	case o.IsWith():
		o.Kids = o.Kids[nl:] // BUG(mistone): how to shorten With ops?
	default:
		return Z(), fmt.Errorf("shorten fail, unknown op: %s", o.String())
	}
	return o, nil
}

func shortenOps(a Op, b Op) (Op, Op, error) {
	var a2, b2 Op
	var err error

	la := a.Len()
	lb := b.Len()

	switch {
	case la == lb:
		return R(0), R(0), nil
	case la > lb:
		a2, err = shorten(a, lb)
		return a2, R(0), err
	case la <= lb:
		b2, err = shorten(b, la)
		return R(0), b2, err
	}
	return Z(), Z(), fmt.Errorf("ot.shortenOps() -- unreachable case, a: %s, b: %s", a, b)
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
		return nil, err
	}
	return Normalize(cs)
}

func compose1(as, bs Ops) (Ops, error) {
	ret := Ops{}
	rest := Ops{}
	hcs := Ops{}
	var err error

	a := 0
	b := 0
	la := len(as)
	lb := len(bs)

	switch {
	case a == la && b == lb:
		break
	case a == la:
		ret = bs.Clone()
	case b == lb:
		ret = as.Clone()
	case la > 0 && as[a].IsZero():
		ret, err = compose1(as[a+1:], bs)
	case lb > 0 && bs[b].IsZero():
		ret, err = compose1(as, bs[b+1:])
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

		sa, sb, err := shortenOps(oa, ob)
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
		case oa.IsInsert() && ob.IsRetain():
			ret = append(ret, Ir(oa.Body[:minlen]))
		case oa.IsInsert() && ob.IsDelete():
			// insertion then deletion cancels
		}
		hcs, err = compose1(has, hbs)
		if err != nil {
			break
		}
		ret = append(ret, hcs...)
	}
	return ret, err
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
	return ret, err
}

func Transform(as, bs Ops) (Ops, Ops, error) {
	var r1, r2 Ops
	var err error

	if bs.Empty() {
		return as.Clone(), bs.Clone(), nil
	}

	r1, r2, err = transform1(as, bs)
	if err != nil {
		return nil, nil, err
	}

	r1, err = Normalize(r1)
	if err != nil {
		return nil, nil, err
	}

	r2, err = Normalize(r2)
	if err != nil {
		return nil, nil, err
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
		case oa.IsDelete() && ob.IsDelete():
		case oa.IsDelete() && ob.IsRetain():
			ra.Delete(minlen)
		case oa.IsRetain() && ob.IsDelete():
			rb.Delete(minlen)
		}
		sa, sb, err = transform1(has, hbs)
		if err != nil {
			break
		}
	default:
		err = fmt.Errorf("transform failed, as: %s, bs: %s", as.String(), bs.String())
		if err != nil {
			break
		}
	}

	ret1 := append(ra, sa...)
	ret2 := append(rb, sb...)
	if err != nil {
		err = fmt.Errorf("transform failed, as: %s, bs: %s\n\tinner err: %s", as.String(), bs.String(), err.Error())
	}
	return ret1, ret2, err
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
			return nil, fmt.Errorf("bad op: %s", o.String())
		}
	}

	return ret2, nil
}

type Doc struct {
	mu sync.Mutex
	// Current text
	body []rune
}

func NewDoc() *Doc {
	d := new(Doc)
	d.body = make([]rune, 0)
	return d
	// return &Doc{
	// 	mu:   sync.Mutex{},
	// 	body: make([]rune{},
	// }
}

func (d *Doc) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	return len(d.body)
}

func (d *Doc) String() string {
	d.mu.Lock()
	defer d.mu.Unlock()

	buf := bytes.Buffer{}
	for _, r := range d.body {
		buf.WriteRune(r)
	}
	return buf.String()
}

func (d *Doc) Apply(os Ops) {
	d.mu.Lock()
	defer d.mu.Unlock()

	os2 := os.Clone()
	os = os2

	if len(os) == 0 {
		return
	}
	pos := 0
	parts := [][]rune{}
	for _, o := range os {
		switch {
		case o.IsDelete():
			pos += o.Len()
		case o.IsRetain() && o.Len() > 0:
			parts = append(parts, d.body[pos:pos+o.Len()])
			pos += o.Len()
		case o.IsInsert():
			parts = append(parts, o.Body)
		}
	}
	size := 0
	for _, p := range parts {
		size += len(p)
	}
	d.body = make([]rune, size)

	idx := 0
	for _, p := range parts {
		copy(d.body[idx:idx+len(p)], p)
		idx += len(p)
	}
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

func I(s string) Op {
	return Op{Tag: O_INSERT, Size: 0, Body: AsRunes(s), Kids: nil}
}

func Ir(r []rune) Op {
	return Op{Tag: O_INSERT, Size: 0, Body: r, Kids: nil}
}

func R(n int) Op {
	return Op{Tag: O_RETAIN, Size: n, Body: nil, Kids: nil}
}

func D(n int) Op {
	if n < 0 {
		n = -n
	}
	return Op{Tag: O_DELETE, Size: -n, Body: nil, Kids: nil}
}

func W(kids Ops) Op {
	return Op{Tag: O_WITH, Size: 0, Body: nil, Kids: kids}
}

func Z() Op {
	return Op{Tag: O_NIL, Size: 0, Body: nil, Kids: nil}
}

func NewInsert(docLen int, pos int, s string) Ops {
	if pos < 0 || pos > docLen+1 {
		panic(fmt.Errorf("bad position; insert is out of range; pos: %d, s: %q", pos, s))
	}

	return Ops{R(pos), I(s), R(docLen - pos)}
}

func NewDelete(docLen int, pos int, length int) Ops {
	if pos < 0 || pos+length > docLen+1 {
		panic(fmt.Errorf("bad position; delete is out of range: pos: %d, len: %d", pos, length))
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
