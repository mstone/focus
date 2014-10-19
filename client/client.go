// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package client uses gopherjs, AngularJS, and the HTTP Focus interface to
// implement a simple Focus client.
package main

import (
	"fmt"

	ng "github.com/gopherjs/go-angularjs"
	"github.com/gopherjs/gopherjs/js"

	"github.com/mstone/focus/js/ace"
	"github.com/mstone/focus/js/alert"
	"github.com/mstone/focus/ot"
)

func main() {
	var aceObj js.Object
	var adapter ace.Adapter
	var state ot.State
	var conn js.Object
	var doc js.Object
	var editor js.Object
	var session js.Object

	// configure Angular
	app := ng.NewModule("root", nil)
	app.NewController("index", func(scope *ng.Scope, interval *ng.Interval, http *ng.HttpService) {

		// configure ACE + attach adapter
		ng.ElementById("editor").Call("ready", func() {
			aceObj = js.Global.Get("ace")
			editor = aceObj.Call("edit", "editor")
			editor.Call("setTheme", "ace/theme/chrome")

			session = editor.Call("getSession")
			session.Call("setMode", "ace/mode/markdown")

			keys := map[string]interface{}{}
			keys["ctrl-t"] = nil
			editor.Get("commands").Call("bindKeys", keys)

			doc = editor.Call("getSession").Call("getDocument")
			doc.Call("setNewLineMode", "unix")

			adapter.AttachEditor(session, doc)
		})

		// configure socket
		conn = js.Global.Get("WebSocket").New("ws://localhost:3000/ws")
		conn.Set("onclose", func(e js.Object) {
			alert.String("WEBSOCKET CLOSED")
		})
		conn.Set("onopen", func(e js.Object) {
			alert.String("WEBSOCKET OPEN")
			state = &ot.Synchronized{}
		})
		conn.Set("onmessage", func(e js.Object) {
			alert.String("WEBSOCKET GOT MSG: " + e.Get("data").Str())
			obj := js.Global.Get("JSON").Call("parse", e.Get("data"))

			rev := obj.Get("Rev").Int()
			ackObj := obj.Get("Ack")
			opsObj := obj.Get("Ops")

			switch {
			default:
				alert.JSON(obj)
				panic("unknown message")
			case !ackObj.IsUndefined() && !ackObj.IsNull() && ackObj.Bool() == true:
				alert.String("ack!")
				state = state.Ack(&adapter, rev)
			case !opsObj.IsUndefined() && !opsObj.IsNull():
				alert.String("write!")
				ops := make(ot.Ops, opsObj.Length())
				for i := 0; i < opsObj.Length(); i++ {
					op := opsObj.Index(i)
					opi := op.Interface()
					switch v := opi.(type) {
					case float64:
						ops[i] = ot.Op{Size: int(v)}
					case string:
						ops[i] = ot.Op{Body: ot.AsSlice(v)}
					}
				}
				state = state.Server(&adapter, rev, ops)
			}
		})

		// acquire site-id + attach adapter to socket
		http.Request(ng.HttpConfig(
			ng.ReqMethod{ng.HttpGet},
			ng.ReqUrl{"/register_site.json"},
		)).Success(func(site *int, status int) {
			alert.String("got site id: " + fmt.Sprintf("%d", *site))
			adapter.AttachSocket(*site, &state, conn)
		}).Error(func(data *string) {
			alert.String("oops -- no site id")
		})
	})
}
