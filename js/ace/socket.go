// Copyright 2015 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package ace

import (
	"encoding/json"

	"github.com/gopherjs/gopherjs/js"

	"github.com/mstone/focus/msg"
)

type SocketSender struct {
	conn *js.Object
}

func NewSocketSender(conn *js.Object) SocketSender {
	return SocketSender{
		conn: conn,
	}
}

func (s SocketSender) Send(msg []byte) {
	s.conn.Call("send", msg)
}

type ReconnectingSocketSender struct {
	conn         *js.Object
	apiEndPoint  string
	vaporpadName string
	onMessage    func(e *js.Object)
}

func NewReconnectingSocketSender(apiEndPoint string, vaporpadName string, onMessage func(e *js.Object)) *ReconnectingSocketSender {
	r := ReconnectingSocketSender{
		conn:         js.Global.Get("WebSocket").New(apiEndPoint),
		apiEndPoint:  apiEndPoint,
		vaporpadName: vaporpadName,
		onMessage:    onMessage,
	}
	r.wireConn()
	return &r
}

func (r *ReconnectingSocketSender) wireConn() {
	r.conn.Set("onclose", r.onClose)
	r.conn.Set("onopen", r.onOpen)
	r.conn.Set("onmessage", r.onMessage)
}

func (r *ReconnectingSocketSender) onClose(e *js.Object) {
	r.conn = js.Global.Get("WebSocket").New(r.apiEndPoint)
	r.wireConn()
}

func (r *ReconnectingSocketSender) onOpen(e *js.Object) {
	jsOps, _ := json.Marshal(msg.Msg{
		Cmd:  msg.C_OPEN,
		Name: r.vaporpadName,
	})
	r.conn.Call("send", jsOps)
}

func (r *ReconnectingSocketSender) Send(msg []byte) {
	r.conn.Call("send", msg)
}
