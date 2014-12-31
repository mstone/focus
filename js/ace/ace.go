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
	"unicode/utf8"

	"github.com/gopherjs/gopherjs/js"

	"github.com/mstone/focus/js/alert"
	"github.com/mstone/focus/msg"
	"github.com/mstone/focus/ot"
)

type Range struct {
	doc       js.Object
	textRange js.Object
}

func NewRange(doc js.Object, textRange js.Object) *Range {
	return &Range{
		doc:       doc,
		textRange: textRange,
	}
}

func (r *Range) asLinearIndex(pos js.Object) int {
	// add the lengths of the lines before startRow, plus
	// startCol, plus 1 * startRow for the line delimiters
	row := pos.Get("row").Int()
	col := pos.Get("column").Int()

	linesBefore := r.doc.Call("getLines", 0, row-1)
	alert.JSON(linesBefore)

	idx := col + row
	for i := 0; i < linesBefore.Length(); i++ {
		idx += linesBefore.Index(i).Get("length").Int()
	}
	return idx
}

func (r *Range) Start() int {
	start := r.textRange.Get("start")
	return r.asLinearIndex(start)
}

func (r *Range) End() int {
	end := r.textRange.Get("end")
	return r.asLinearIndex(end)
}

func RowCol(doc js.Object, pos int) js.Object {
	var row, col int
	lines := doc.Call("getAllLines")
	alert.String("lines")
	alert.JSON(lines)
	for i := 0; i < lines.Length(); i++ {
		lineLen := lines.Index(i).Length()
		if pos <= lineLen {
			row = i
			col = pos
			break
		} else {
			pos -= lineLen + 1
		}
	}
	obj := js.Global.Get("Object").New()
	obj.Set("row", row)
	obj.Set("column", col)
	return obj
}

func StartEnd(doc js.Object, start, end int) js.Object {
	ret := js.Global.Get("Object").New()
	ret.Set("start", RowCol(doc, start))
	ret.Set("end", RowCol(doc, end))
	return ret
}

type Adapter struct {
	conn     js.Object
	session  js.Object
	doc      js.Object
	state    *ot.State
	suppress bool
	rev      int
}

func NewAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Suppress(suppress bool) {
	a.suppress = suppress
}

func (a *Adapter) IsSuppressed() bool {
	return a.suppress
}

func (a *Adapter) Send(ops ot.Ops) {
	if !a.suppress {
		jsOps, _ := json.Marshal(msg.Msg{
			Cmd: msg.C_WRITE,
			Rev: a.rev,
			Ops: ops,
		})
		alert.Golang(fmt.Sprintf("sending jsops: %s", jsOps))
		a.conn.Call("send", jsOps)
	}
}

func (a *Adapter) Ack(rev int) {
	a.rev = rev
}

func (a *Adapter) Recv(rev int, ops ot.Ops) {
	pos := 0
	alert.String(fmt.Sprintf("recv(%s)", ops.String()))

	a.suppress = true
	defer func() {
		a.suppress = false
	}()

	for _, op := range ops {
		if op.Len() == 0 {
			continue
		}
		switch {
		case op.IsRetain():
			alert.String(fmt.Sprintf("retain(%d)", op.Size))
			pos += op.Size
			continue
		case op.IsInsert():
			rowcol := RowCol(a.doc, pos)
			alert.String(fmt.Sprintf("insert(%d, %q)", pos, ot.AsString(op.Body)))
			alert.JSON(rowcol)
			a.doc.Call("insert", rowcol, ot.AsString(op.Body))
			continue
		case op.IsDelete():
			startEnd := StartEnd(a.doc, pos, pos-op.Size)
			alert.String(fmt.Sprintf("remove(%d, %d)", pos, pos-op.Size))
			a.doc.Call("remove", startEnd)
		}
	}

	a.rev = rev
}

func (a *Adapter) AttachEditor(session js.Object, doc js.Object) {
	a.session = session
	a.doc = doc
	doc.Call("on", "change", a.OnChange)
}

func (a *Adapter) AttachSocket(state *ot.State, conn js.Object) {
	a.state = state
	a.conn = conn
}

func (a *Adapter) OnChange(change js.Object) bool {
	if a.IsSuppressed() {
		alert.String("change SUPPRESSED")
		return true
	}
	length := a.session.Call("getValue").Length()
	alert.String(fmt.Sprintf("session len: %d", length))

	data := change.Get("data")
	alert.JSON(data)

	action := data.Get("action").Str()
	textRange := NewRange(a.doc, data.Get("range"))

	start := textRange.Start()
	end := textRange.End()

	var ops ot.Ops
	switch action {
	case "insertText":
		str := data.Get("text").Str()
		oldLen := length - (end - start)
		alert.String(fmt.Sprintf("newInsert(%d, %d, %q)", oldLen, start, str))
		ops = ot.NewInsert(oldLen, start, str)
	case "removeText":
		oldLen := length + (end - start)
		alert.String(fmt.Sprintf("newDelete(%d, %d, %d)", oldLen, start, end-start))
		ops = ot.NewDelete(oldLen, start, end-start)
	case "insertLines":
		linesObj := data.Get("lines")
		numLines := linesObj.Length()
		lines := make([]string, numLines)
		for i := 0; i < numLines; i++ {
			lines[i] = linesObj.Index(i).Str() + "\n"
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
			lines[i] = linesObj.Index(i).Str() + "\n"
		}
		str := strings.Join(lines, "")
		numRunes := utf8.RuneCountInString(str)
		oldLen := length + numRunes
		ops = ot.NewDelete(oldLen, start, numRunes)
	}

	alert.String("sending ops")
	alert.Golang(ops)

	*a.state = (*a.state).Client(a, ops)
	return true
}
