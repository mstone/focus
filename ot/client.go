// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.
//
// Note: this file is a derivative work of
//
// https://github.com/Operational-Transformation/ot.js/blob/master/lib/client.js
//
// Copyright © 2012-2014 Tim Baumann, http://timbaumann.info
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
	"fmt"
)

type State interface {
	Client(c Client, ops Ops) State
	Server(c Client, rev int, ops Ops) State
	Ack(c Client, rev int) State
	String() string
}

type Client interface {
	Send(ops Ops)
	Recv(rev int, ops Ops)
	Ack(rev int)
}

type Synchronized struct{}

var synchronized = &Synchronized{}

func (s *Synchronized) String() string {
	return "Synchronized{}"
}

func (s *Synchronized) Client(c Client, ops Ops) State {
	c.Send(ops)
	return &Waiting{
		inflight: ops,
	}
}

func (s *Synchronized) Server(c Client, rev int, ops Ops) State {
	c.Recv(rev, ops)
	return s
}

func (s *Synchronized) Ack(c Client, rev int) State {
	panic("ack received while synched")
}

type Waiting struct {
	inflight Ops
}

func (w *Waiting) String() string {
	return fmt.Sprintf("Waiting{inflight: %s}", w.inflight)
}

func (w *Waiting) Client(c Client, ops Ops) State {
	return &Buffering{
		inflight: w.inflight,
		waiting:  ops,
	}
}

func (w *Waiting) Server(c Client, rev int, ops Ops) State {
	inflight2, ops2 := Transform(w.inflight, ops)
	c.Recv(rev, ops2)
	return &Waiting{
		inflight: inflight2,
	}
}

func (w *Waiting) Ack(c Client, rev int) State {
	c.Ack(rev)
	return synchronized
}

type Buffering struct {
	inflight Ops
	waiting  Ops
}

func (b *Buffering) String() string {
	return fmt.Sprintf("Buffering{inflight: %s, waiting: %s}", b.inflight, b.waiting)
}

func (b *Buffering) Client(c Client, ops Ops) State {
	return &Buffering{
		inflight: b.inflight,
		waiting:  Compose(b.waiting, ops),
	}
}

func (b *Buffering) Server(c Client, rev int, ops Ops) State {
	/*
	        *
	      i/ \o
	      *   *
	    w/ \ /i2
	    *   *
	   o3\ /w2
	      *
	*/

	i := b.inflight
	o := ops
	w := b.waiting

	i2, o2 := Transform(i, o)
	w2, o3 := Transform(w, o2)

	c.Recv(rev, o3)
	return &Buffering{
		inflight: i2,
		waiting:  w2,
	}
}

func (b *Buffering) Ack(c Client, rev int) State {
	c.Ack(rev)
	c.Send(b.waiting)
	return &Waiting{
		inflight: b.waiting,
	}
}

func NewInsert(docLen int, pos int, s string) Ops {
	if pos < 0 || pos > docLen+1 {
		panic(fmt.Errorf("bad position; insert is out of range; pos: %d, s: %q", pos, s))
	}

	runes := AsRunes(s)

	os := Ops{
		Op{
			Size: pos,
			Body: nil,
		},
		Op{
			Size: 0,
			Body: runes,
		},
		Op{
			Size: docLen - pos,
			Body: nil,
		},
	}

	return os
}

func NewDelete(docLen int, pos int, length int) Ops {
	if pos < 0 || pos+length > docLen+1 {
		panic(fmt.Errorf("bad position; delete is out of range: pos: %d, len: %d", pos, length))
	}

	os := Ops{
		Op{
			Size: pos,
			Body: nil,
		},
		Op{
			Size: -length,
			Body: nil,
		},
		Op{
			Size: docLen - length - pos,
			Body: nil,
		},
	}

	return os
}
