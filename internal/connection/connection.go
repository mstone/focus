// Copyright 2015 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package connection

import (
	"sync"
	"time"

	im "github.com/mstone/focus/internal/msgs"
	"github.com/mstone/focus/msg"
)

type WebSocket interface {
	ReadJSON(v interface{}) error
	WriteJSON(v interface{}) error
	SetReadTimeout(d time.Duration) error
	SetWriteTimeout(d time.Duration) error
	CancelReadTimeout() error
	CancelWriteTimeout() error
}

// struct conn represents an open WebSocket connection.
type conn struct {
	mu   sync.Mutex
	msgs chan interface{}
	ws   WebSocket
	docs map[int]chan interface{}
	srvr chan interface{}
}

func New(srvr chan interface{}, ws WebSocket) chan interface{} {
	c := &conn{
		mu:   sync.Mutex{},
		msgs: make(chan interface{}),
		ws:   ws,
		docs: map[int]chan interface{}{},
		srvr: srvr,
	}
	go c.readLoop()
	go c.writeLoop()
	return c.msgs
}

func (c *conn) Close() error {
	return nil
}

func (c *conn) onVppOpen(m msg.Msg) {
	srvrReplyChan := make(chan im.Allocdocresp)
	c.srvr <- im.Allocdoc{
		Reply: srvrReplyChan,
		Name:  m.Name,
	}

	srvrResp := <-srvrReplyChan
	if srvrResp.Err != nil {
		panic("conn unable to Allocdoc")
	}

	complete := make(chan im.Opencompletion)

	doc := srvrResp.Doc
	doc <- im.Open{
		Reply: complete,
		Conn:  c.msgs,
		Name:  m.Name,
	}

	cmp := <-complete
	c.setDoc(cmp.Fd, cmp.Doc)
}

func (c *conn) getDoc(fd int) (chan interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	doc, ok := c.docs[fd]

	return doc, ok
}

func (c *conn) setDoc(fd int, doc chan interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.docs[fd] = doc
}

func (c *conn) onVppWrite(m msg.Msg) {
	doc, ok := c.getDoc(m.Fd)
	if !ok {
		panic("conn got WRITE with bad fd")
	}
	doc <- im.Write{
		Fd:  m.Fd,
		Rev: m.Rev,
		Ops: m.Ops,
	}
}

func (c *conn) readLoop() {
	for {
		var m msg.Msg

		if err := c.ws.ReadJSON(&m); err != nil {
			c.Close() // BUG(mistone): errcheck?
			return
		}

		switch m.Cmd {
		default:
			return
		case msg.C_OPEN:
			c.onVppOpen(m)
		case msg.C_WRITE:
			c.onVppWrite(m)
		}
	}
}

func (c *conn) writeLoop() {
	for m := range c.msgs {
		switch v := m.(type) {
		case im.Openresp:
			c.ws.WriteJSON(msg.Msg{
				Cmd:  msg.C_OPEN_RESP,
				Name: v.Name,
				Fd:   v.Fd,
			})
		case im.Writeresp:
			c.ws.WriteJSON(msg.Msg{
				Cmd: msg.C_WRITE_RESP,
				Fd:  v.Fd,
				Rev: v.Rev,
			})
		case im.Write:
			c.ws.WriteJSON(msg.Msg{
				Cmd: msg.C_WRITE,
				Fd:  v.Fd,
				Rev: v.Rev,
				Ops: v.Ops,
			})
		}
	}
}
