// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package client uses gopherjs, AngularJS, and the HTTP Focus interface to
// implement a simple Focus client.
package main

import (
	"encoding/json"
	"github.com/gopherjs/gopherjs/js"
	"github.com/mstone/focus/msg"

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

	// configure ACE + attach adapter
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

	// configure socket
	conn = js.Global.Get("WebSocket").New("ws://localhost:3000/ws")
	conn.Set("onclose", func(e js.Object) {
	})
	conn.Set("onopen", func(e js.Object) {
		state = &ot.Synchronized{}
		jsOps, _ := json.Marshal(msg.Msg{
			Cmd:  msg.C_OPEN,
			Name: "index.txt",
		})
		conn.Call("send", jsOps)
	})
	conn.Set("onmessage", func(e js.Object) {
		m := msg.Msg{}

		err := json.Unmarshal([]byte(e.Get("data").Str()), &m)
		if err != nil {
			alert.Golang(err)
			panic(err.Error())
		}

		switch m.Cmd {
		default:
			alert.Golang(m)
			panic("unknown message")
		case msg.C_OPEN_RESP:
			adapter.AttachFd(m.Fd)
		case msg.C_WRITE_RESP:
			state = state.Ack(&adapter, m.Rev)
		case msg.C_WRITE:
			state = state.Server(&adapter, m.Rev, m.Ops)
		}
	})

	adapter.AttachSocket(&state, conn)
}
