// Copyright 2016 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package ace

import (
	"github.com/gopherjs/gopherjs/js"
)

type Position interface {
	Row() int
	Col() int
	Set(row, col int)
	JS() *js.Object
}

type JSPosition struct {
	pos *js.Object
}

func (j JSPosition) Row() int {
	return j.pos.Get("row").Int()
}

func (j JSPosition) Col() int {
	return j.pos.Get("column").Int()
}

func (j JSPosition) Set(row, col int) {
	j.pos.Set("row", row)
	j.pos.Set("column", col)
}

func (j JSPosition) JS() *js.Object {
	return j.pos
}

func NewRowCol(doc Document, pos int) Position {
	var row, col int
	lines := doc.GetAllLines()
	// alert.String("lines")
	// alert.JSON(lines)
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
	obj := JSPosition{js.Global.Get("Object").New()}
	obj.Set(row, col)
	return obj
}
