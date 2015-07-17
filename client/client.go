// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package client uses gopherjs, AngularJS, and the HTTP Focus interface to
// implement a simple Focus client.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/gopherjs/gopherjs/js"

	"github.com/mstone/focus/msg"
	"github.com/mstone/focus/js/ace"
	"github.com/mstone/focus/js/alert"
	"github.com/mstone/focus/ot"
)

func makeGetElementById(id string, obj **js.Object) (built bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case *js.Error:
				*obj = nil
				built = true
				err = v
			default:
				*obj = nil
				built = true
				err = fmt.Errorf("unknown error while getting editor element; err: %q", r)
			}
		}
	}()

	// stale?
	if *obj == nil {
		// rebuild.
		*obj = js.Global.Get("document").Call("getElementById", id)
		built = true
		err = nil
	} else {
		built = false
		err = nil
	}
	return built, err
}

func main() {
	var aceDiv *js.Object
	var aceObj *js.Object
	var adapter *ace.Adapter
	var state *ot.Controller
	var conn *js.Object
	var doc *js.Object
	var editor *js.Object
	var session *js.Object

	// find editor element
	// find ace obj
	// create editor object
	// wire up editor object to editor element
	// configure editor object
	// find session object
	// configure session object
	// configure editor keys setting
	// find editor-document object
	// set document line mode
	// create session lengther object from session
	// create OT adapter
	// create adapter-doc object wrapping editor-doc
	// wire adapter to the session lengther and to a adapter-doc
	// get api endpoint
	// create ot-controller using adapter
	// create websocket
	// wire websocket event handlers
	// create new socketsender from websocket
	// attach socketsender to controller and adapter



	// configure ACE + attach adapter
	//aceDiv = js.Global.Get("document").Call("getElementById", "editor")
	built, err := makeGetElementById("editor", &aceDiv)
	if err != nil {
		panic(fmt.Errorf("unable to get #editor, err: %q", err))
	}
	if !built {
		panic("surprise; #editor not rebuilt!")
	}


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

	sessionLengther := ace.NewSessionLengther(session)

	adapter = ace.NewAdapter()
	adapter.AttachEditor(sessionLengther, ace.NewJSDocument(doc))

	// configure socket
	apiEndPoint := aceDiv.Get("dataset").Get("vppApi")
	vaporpadName := aceDiv.Get("dataset").Get("vppName")

	state = ot.NewController(adapter, adapter)
	conn = js.Global.Get("WebSocket").New(apiEndPoint.String())

	conn.Set("onclose", func(e *js.Object) {
	})
	conn.Set("onopen", func(e *js.Object) {

		jsOps, _ := json.Marshal(msg.Msg{
			Cmd:  msg.C_OPEN,
			Name: vaporpadName.String(),
		})
		go func() {
			conn.Call("send", jsOps)
		}()
	})
	conn.Set("onmessage", func(e *js.Object) {
		m := msg.Msg{}

		err := json.Unmarshal([]byte(e.Get("data").String()), &m)
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
			state.OnServerAck(m.Rev, m.Ops)
		case msg.C_WRITE:
			state.OnServerWrite(m.Rev, m.Ops)
		}
	})

	connSender := ace.NewSocketSender(conn)

	adapter.AttachSocket(state, connSender)
}
