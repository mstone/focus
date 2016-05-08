// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.
//
// Note: this file is a derivative work of
//
// https://github.com/share/ShareJS/blob/0.6/src/client/ace.coffee
//
// Copyright 2011 Joseph Gentle
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package ace

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/gopherjs/gopherjs/js"

	"github.com/mstone/focus/js/alert"
	"github.com/mstone/focus/msg"
	"github.com/mstone/focus/ot"
)

type Lengther interface {
	Length() int
}

type Sender interface {
	Send(msg []byte)
}

type Adapter struct {
	mu       sync.Mutex
	conn     Sender
	session  Lengther
	doc      Document
	state    *ot.Controller
	suppress bool
	fd       int
}

func NewAdapter() *Adapter {
	return &Adapter{
		mu: sync.Mutex{},
	}
}

func (a *Adapter) Suppress(suppress bool) {
	a.suppress = suppress
}

func (a *Adapter) IsSuppressed() bool {
	return a.suppress
}

func (a *Adapter) Send(rev int, hash string, ops ot.Ops) {
	if !a.suppress {
		jsOps, _ := json.Marshal(msg.Msg{
			Cmd:  msg.C_WRITE,
			Fd:   a.fd,
			Rev:  rev,
			Hash: hash,
			Ops:  ops,
		})
		alert.Golang(fmt.Sprintf("sending jsops: %s", jsOps))
		a.conn.Send(jsOps)
	}
}

func (a *Adapter) Recv(ops ot.Ops) {
	a.mu.Lock()
	defer a.mu.Unlock()

	pos := 0
	alert.String(fmt.Sprintf("recv(%s)", ops.String()))

	a.suppress = true
	defer func() {
		a.suppress = false
	}()

	if len(ops) == 0 {
		return
	}
	if len(ops) != 1 || !ops[0].IsWith() {
		alert.String("recv err; ops len != 1 || ops[0] != With(...)")
		panic("bad ops len")
	}
	o := ops[0]

	for _, op := range o.Kids {
		switch {
		case op.IsZero():
			continue
		case op.IsInsert():
			rowcol := NewRowCol(a.doc, pos)
			alert.String(fmt.Sprintf("insert(%d, %q)", pos, op.Body.String()))
			a.doc.Insert(rowcol, ot.AsString([]rune{op.Body.Leaf}))
			pos += op.Len()
			continue
		case op.IsRetain():
			alert.String(fmt.Sprintf("retain(%d)", op.Size))
			pos += op.Size
			continue
		case op.IsDelete():
			startEnd := NewStartEnd(a.doc, pos, pos-op.Size)
			alert.String(fmt.Sprintf("remove(%d, %d)", pos, pos-op.Size))
			a.doc.Remove(startEnd)
		case op.IsWith():
			alert.String("recv err; got inner with op; exiting")
			panic(2)
		}
	}
}

func (a *Adapter) AttachFd(fd int) {
	a.fd = fd
}

func (a *Adapter) AttachEditor(session Lengther, doc Document) {
	a.session = session
	a.doc = doc
	doc.SetOnChange(a.OnChange)
}

func (a *Adapter) AttachSocket(state *ot.Controller, conn Sender) {
	a.state = state
	a.conn = conn
}

// RowCall(...)
//   -> getAllLines() -> Array[Line]
// StartEnd(...)
//   -> RowCol
// NewRange(...)
//   -> getLines(a, b)  -> Array[Line]  (.Get("length").Int())
// On("change")
// Insert()
// Remove()

func (a *Adapter) OnChange(change *js.Object) bool {
	if a.IsSuppressed() {
		// alert.String("change SUPPRESSED")
		return true
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	length := a.session.Length()
	alert.String(fmt.Sprintf("session len: %d", length))

	data := change.Get("data")
	alert.JSON(data)

	action := data.Get("action").String()
	textRange := NewRange(a.doc, NewJSStartEnd(data.Get("range")))

	start := textRange.Start()
	end := textRange.End()

	var ops ot.Ops
	switch action {
	case "insertText":
		str := data.Get("text").String()
		oldLen := length - (end - start)
		alert.String(fmt.Sprintf("newInsert(%d, %d, %q)", oldLen, start, str))
		ops = ot.NewInsert(oldLen, start, str)
	case "removeText":
		// BUG(mistone): end is bogus after delete when \n in text...
		// Repro: abc\ndef<<<^H
		text := data.Get("text").String()
		textLen := len(text)
		oldLen := length + textLen
		alert.String(fmt.Sprintf("newDelete(%d, %d, %d)", oldLen, start, textLen))
		ops = ot.NewDelete(oldLen, start, textLen)
	case "insertLines":
		linesObj := data.Get("lines")
		numLines := linesObj.Length()
		lines := make([]string, numLines)
		for i := 0; i < numLines; i++ {
			lines[i] = linesObj.Index(i).String() + "\n"
		}
		str := strings.Join(lines, "")
		numRunes := utf8.RuneCountInString(str)
		oldLen := length - numRunes
		ops = ot.NewInsert(oldLen, start, str)
	case "removeLines":
		linesObj := data.Get("lines")
		numLines := linesObj.Length()
		lines := make([]string, numLines)
		for i := 0; i < numLines; i++ {
			lines[i] = linesObj.Index(i).String() + "\n"
		}
		str := strings.Join(lines, "")
		numRunes := utf8.RuneCountInString(str)
		oldLen := length + numRunes
		ops = ot.NewDelete(oldLen, start, numRunes)
	}

	ops = ot.Ws(ops)
	alert.String("sending ops")
	alert.Golang(ops)

	go a.state.OnClientWrite(ops)
	return true
}
