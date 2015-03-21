package ace

import (
	"github.com/gopherjs/gopherjs/js"
)

type SessionLengther struct {
	session *js.Object
}

func (a SessionLengther) Length() int {
	return a.session.Call("getValue").Length()
}

func NewSessionLengther(session *js.Object) SessionLengther {
	return SessionLengther{
		session: session,
	}
}
