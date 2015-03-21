package ace

import (
	"github.com/gopherjs/gopherjs/js"
)

type Document interface {
	GetLines(a, b int) LineArray
	GetAllLines() LineArray
	SetOnChange(f func(ev *js.Object) bool)
	Insert(p Position, s string)
	Remove(se StartEnd)
}

type JSDocument struct {
	doc *js.Object
}

func (j JSDocument) GetLines(a, b int) LineArray {
	return JSLineArray{j.doc.Call("getLines", a, b)}
}

func (j JSDocument) GetAllLines() LineArray {
	return JSLineArray{j.doc.Call("getAllLines")}
}

func (j JSDocument) SetOnChange(f func(ev *js.Object) bool) {
	j.doc.Call("on", "change", f)
}

func (j JSDocument) Insert(p Position, s string) {
	j.doc.Call("insert", p.JS(), s)
}

func (j JSDocument) Remove(se StartEnd) {
	j.doc.Call("remove", se.JS())
}

func NewJSDocument(doc *js.Object) JSDocument {
	return JSDocument{
		doc: doc,
	}
}
