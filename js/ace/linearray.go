package ace

import (
	"github.com/gopherjs/gopherjs/js"
)

type LineArray interface {
	Length() int
	Index(i int) Line
}

type JSLineArray struct {
	arr *js.Object
}

func (j JSLineArray) Length() int {
	return j.arr.Length()
}

func (j JSLineArray) Index(i int) Line {
	return JSLine{j.arr.Index(i)}
}
