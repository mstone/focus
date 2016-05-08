// Copyright 2016 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package ace

import (
	"github.com/gopherjs/gopherjs/js"
)

type StartEnd interface {
	Start() Position
	End() Position
	Set(start, end Position)
	JS() *js.Object
}

type JSStartEnd struct {
	obj *js.Object
}

func (j JSStartEnd) Start() Position {
	return JSPosition{j.obj.Get("start")}
}

func (j JSStartEnd) End() Position {
	return JSPosition{j.obj.Get("end")}
}

func (j JSStartEnd) Set(start, end Position) {
	j.obj.Set("start", start.JS())
	j.obj.Set("end", end.JS())
}

func (j JSStartEnd) JS() *js.Object {
	return j.obj
}

func NewStartEnd(doc Document, start, end int) StartEnd {
	ret := JSStartEnd{js.Global.Get("Object").New()}
	ret.Set(NewRowCol(doc, start), NewRowCol(doc, end))
	return ret
}

func NewJSStartEnd(obj *js.Object) StartEnd {
	return JSStartEnd{
		obj: obj,
	}
}
