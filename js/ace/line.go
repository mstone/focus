// Copyright 2016 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package ace

import (
	"github.com/gopherjs/gopherjs/js"
)

type Line interface {
	Length() int
}

type JSLine struct {
	line *js.Object
}

func (j JSLine) Length() int {
	return j.line.Length()
}
