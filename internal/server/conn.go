package server

import (
	"fmt"
	"github.com/mstone/focus/msg"
	"sync"
	"time"

	log "gopkg.in/inconshreveable/log15.v2"
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
	msgs    chan interface{}
	l       log.Logger
	no      int
	numSend int
	numRecv int
	wg      sync.WaitGroup
	ws      WebSocket
	docs    map[int]chan interface{}
	srvr    chan interface{}
}

func (c *conn) String() string {
	if c == nil {
		return "nil"
	}
	return fmt.Sprintf("%d", c.no)
}

func (c *conn) Run() {
	c.wg.Add(2)
	go c.readLoop()
	go c.writeLoop()
	c.wg.Wait()
}

func (c *conn) Close() error {
	return nil
}

func (c *conn) onVppOpen(m msg.Msg) {
	srvrReplyChan := make(chan allocdocresp)
	c.srvr <- allocdoc{
		reply: srvrReplyChan,
		name:  m.Name,
	}

	srvrResp := <-srvrReplyChan
	if srvrResp.err != nil {
		panic("conn unable to Allocdoc")
	}

	doc := srvrResp.doc
	doc <- open{
		dbgConn: c,
		conn:    c.msgs,
		name:    m.Name,
	}

}

func (c *conn) onVppWrite(m msg.Msg) {
	doc, ok := c.docs[m.Fd]
	if !ok {
		panic("conn got WRITE with bad fd")
	}
	doc <- write{
		dbgConn: c,
		fd:      m.Fd,
		rev:     m.Rev,
		ops:     m.Ops,
	}
}

func (c *conn) readLoop() {
	defer c.wg.Done()

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
		c.numRecv++
	}
}

func (c *conn) writeLoop() {
	defer c.wg.Done()

	for m := range c.msgs {
		switch v := m.(type) {
		case openresp:
			c.docs[v.fd] = v.doc
			c.ws.WriteJSON(msg.Msg{
				Cmd:  msg.C_OPEN_RESP,
				Name: v.name,
				Fd:   v.fd,
			})
		case writeresp:
			c.ws.WriteJSON(msg.Msg{
				Cmd: msg.C_WRITE_RESP,
				Fd:  v.fd,
				Rev: v.rev,
			})
		case write:
			c.ws.WriteJSON(msg.Msg{
				Cmd: msg.C_WRITE,
				Fd:  v.fd,
				Rev: v.rev,
				Ops: v.ops,
			})
		}
		c.numSend++
	}
}
