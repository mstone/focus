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
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

type Tag int

type Op struct {
	// Len is either delete-len (if negative) or retain-len (if positive)
	Size int

	// Body is the text to be inserted
	Body []rune
}

func (o Op) Clone() Op {
	var body2 []rune
	if len(o.Body) > 0 {
		body2 = make([]rune, len(o.Body))
		copy(body2, o.Body)
	}
	return Op{
		Size: o.Size,
		Body: body2,
	}
}

func (o *Op) IsRetain() bool {
	if o == nil {
		return false
	}
	return o.Size > 0
}

func (o *Op) IsDelete() bool {
	if o == nil {
		return false
	}
	return o.Size < 0
}

func (o *Op) IsInsert() bool {
	if o == nil {
		return false
	}
	return o.Size == 0
	// return o.Size == 0 && len(o.Body) > 0
}

func (o *Op) Len() int {
	switch {
	case o == nil:
		return 0
	case o.Size < 0:
		return -o.Size
	case o.Size > 0:
		return o.Size
	default:
		return len(o.Body)
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
		return "N "
	case o.IsDelete():
		return fmt.Sprintf("D%d", -o.Size)
	case o.IsRetain():
		return fmt.Sprintf("R%d", o.Size)
	case o.IsInsert():
		return fmt.Sprintf("I%s", AsString(o.Body))
	default:
		return fmt.Sprintf("E%#v", o)
	}
}

type Ops []Op

func (os Ops) Clone() Ops {
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
			os.insertPenultimate(Op{Body: body})
		}
	default:
		os.insertUltimate(Op{Body: body})
	}
}

func (os *Ops) Retain(size int) {
	switch {
	case size == 0:
		return
	case len(*os) > 0 && os.Last().IsRetain():
		os.Last().Size += size
	default:
		os.insertUltimate(Op{Size: size})
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
		os.insertUltimate(Op{Size: size})
	}
}

func (o Op) MarshalJSON() ([]byte, error) {
	switch {
	case o.IsDelete() || o.IsRetain():
		return json.Marshal(o.Size)
	default:
		return json.Marshal(AsString(o.Body))
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
		return nil
	default:
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

func (o Op) Shorten(nl int) Op {
	switch {
	case o.IsRetain():
		o.Size -= nl
	case o.IsDelete():
		o.Size += nl
	case o.IsInsert():
		o.Body = o.Body[nl:]
	}
	return o
}

func shortenOps(a *Op, b *Op) (*Op, *Op) {
	la := a.Len()
	lb := b.Len()
	switch {
	case la == lb:
		return nil, nil
	case a != nil && b != nil && la > lb:
		a2 := a.Shorten(lb)
		return &a2, nil
	case a != nil && b != nil && la <= lb:
		b2 := b.Shorten(la)
		return nil, &b2
	}
	return a, b
}

func Compose(as, bs Ops) Ops {
	ops := Ops{}

	if len(as) == 0 {
		ops := make(Ops, len(bs))
		copy(ops, bs)
		return ops
	}

Fix:
	for {
		switch {
		case len(as) == 0 && len(bs) == 0:
			break Fix
		case len(as) > 0 && as.First().IsDelete():
			ops.Delete(as.First().Size)
			as = as.Rest()
			continue
		case len(bs) > 0 && bs.First().IsInsert():
			ops.Insert(bs.First().Body)
			bs = bs.Rest()
			continue
		case len(as) > 0 && len(bs) > 0:
			a := as.First()
			b := bs.First()
			minlen := min(a.Len(), b.Len())
			switch {
			case a.IsRetain() && b.IsRetain():
				ops.Retain(minlen)
			case a.IsRetain() && b.IsDelete():
				ops.Delete(minlen)
			case a.IsInsert() && b.IsRetain():
				ops.Insert(a.Body[:minlen])
			case a.IsInsert() && b.IsDelete():
				//ops.Delete(minlen)
			}
			a, b = shortenOps(a, b)
			if a == nil {
				as = as.Rest()
			} else {
				as = append(Ops{*a}, as.Rest()...)
			}
			if b == nil {
				bs = bs.Rest()
			} else {
				bs = append(Ops{*b}, bs.Rest()...)
			}
			continue
		case len(as) > 0 && len(bs) == 0:
			ops = append(ops, as...)
			as = nil
			continue
		case len(as) == 0 && len(bs) > 0:
			ops = append(ops, bs...)
			bs = nil
			continue
		default:
			panic("impossible")
		}
	}

	return ops
}

func Transform(as, bs Ops) (Ops, Ops) {
	var ai, bi int
	var al, bl int
	var aos, bos Ops
	var a, b *Op

	al = len(as)
	bl = len(bs)

	if ai < al {
		a = &as[ai]
	}
	if bi < bl {
		b = &bs[bi]
	}

	if al == 0 {
		return nil, bs.Clone()
	}
	if bl == 0 {
		return as.Clone(), nil
	}

	var loopCount = -1

	// fmt.Printf("xform: as: %s, bs: %s, a: %s, b: %s, ai: %d, bi: %d\n", as, bs, a, b, ai, bi)
	for {
		loopCount++
		// fmt.Printf("loop: %d, a: %s, b: %s, ai: %d, bi: %d\n", loopCount, a, b, ai, bi)

		if a == nil && b == nil {
			break
		}

		if a.IsInsert() {
			aos.Insert(a.Body)
			bos.Retain(a.Len())
			ai++
			if ai < al {
				a = &as[ai]
			} else {
				a = nil
			}
			continue
		}
		if b.IsInsert() {
			aos.Retain(b.Len())
			bos.Insert(b.Body)
			bi++
			if bi < bl {
				b = &bs[bi]
			} else {
				b = nil
			}
			continue
		}
		if a == nil {
			panic("boom1")
		}
		if b == nil {
			panic("boom2")
		}

		minlen := min(a.Len(), b.Len())
		switch {
		case a.IsRetain() && b.IsRetain():
			aos.Retain(minlen)
			bos.Retain(minlen)
		case a.IsDelete() && b.IsRetain():
			aos.Delete(minlen)
		case a.IsRetain() && b.IsDelete():
			bos.Delete(minlen)
		}
		// a2, b2 := a, b
		a, b = shortenOps(a, b)
		// fmt.Printf("shorten:\n\ta : %s\n\tb : %s\n\ta2: %s\n\tb2: %s\n", a2, b2, a, b)
		if a == nil {
			ai++
			if ai < al {
				a = &as[ai]
			} else {
				a = nil
			}
		}
		if b == nil {
			bi++
			if bi < bl {
				b = &bs[bi]
			} else {
				b = nil
			}
		}
	}

	return aos, bos
}

func Normalize(os Ops) Ops {
	os2 := Ops{}
	for _, o := range os {
		if o.Len() != 0 {
			os2 = append(os2, o)
		}
	}
	return os2
}

type Doc struct {
	// Current text
	body []rune
}

func NewDoc() *Doc {
	return &Doc{
		body: []rune{},
	}
}

func (d *Doc) Len() int {
	return len(d.body)
}

func (d *Doc) String() string {
	buf := bytes.Buffer{}
	for _, r := range d.body {
		buf.WriteRune(r)
	}
	return buf.String()
}

func (d *Doc) Apply(os Ops) {
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
