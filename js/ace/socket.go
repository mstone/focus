package ace

import (
	"github.com/gopherjs/gopherjs/js"
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
