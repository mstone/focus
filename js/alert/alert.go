// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package alert

import (
	"fmt"

	"github.com/gopherjs/gopherjs/js"
)

func Golang(s interface{}) {
	if js.Global == nil {
		fmt.Printf("%+v\n", s)
	} else {
		js.Global.Get("console").Call("log", fmt.Sprintf("%+v", s))
	}
}

func String(s string) {
	if js.Global == nil {
		fmt.Printf("%s\n", s)
	} else {
		js.Global.Get("console").Call("log", s)
	}
}

func JSON(o *js.Object) {
	if js.Global == nil {
		fmt.Printf("%s\n", o.String())
	} else {
		s := o.String()
		// js.Global.Get("JSON").Call("stringify", o).String()
		js.Global.Get("console").Call("log", s)
	}
}
