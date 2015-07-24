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
	mu     sync.Mutex
	msgs   chan interface{}
	ws     WebSocket
	docs   map[int]chan interface{}
	fds    map[chan interface{}]int
	srvr   chan interface{}
	nextFd int
}

func New(srvr chan interface{}, ws WebSocket) chan interface{} {
	c := &conn{
		mu:     sync.Mutex{},
		msgs:   make(chan interface{}),
		ws:     ws,
		docs:   map[int]chan interface{}{},
		fds:    map[chan interface{}]int{},
		srvr:   srvr,
		nextFd: 0,
	}
	go c.readLoop()
	go c.writeLoop()
	return c.msgs
}

func (c *conn) Close() error {
	return nil
}

func (c *conn) allocFd() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	fd := c.nextFd

	c.nextFd++

	return fd
}

func (c *conn) getDoc(fd int) (chan interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	doc, ok := c.docs[fd]
	return doc, ok
}

func (c *conn) getFd(doc chan interface{}) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fd, ok := c.fds[doc]
	return fd, ok
}

func (c *conn) setDoc(fd int, doc chan interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.docs[fd] = doc
	c.fds[doc] = fd
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

	fd := c.allocFd()
	doc := srvrResp.Doc
	c.setDoc(fd, doc)

	doc <- im.Open{
		Conn: c.msgs,
		Name: m.Name,
		Fd:   fd,
		Rev:  m.Rev,
	}
}

func (c *conn) onVppWrite(m msg.Msg) {
	doc, ok := c.getDoc(m.Fd)
	if !ok {
		panic("conn got WRITE with bad fd")
	}
	doc <- im.Write{
		Conn: c.msgs,
		Rev:  m.Rev,
		Hash: m.Hash,
		Ops:  m.Ops.Clone(),
	}
}

func (c *conn) readLoop() {
	for {
		m := msg.Msg{}

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
			fd, ok := c.getFd(v.Doc)
			if !ok {
				panic("conn got WRITERESP with bad doc")
			}
			c.ws.WriteJSON(msg.Msg{
				Cmd: msg.C_WRITE_RESP,
				Fd:  fd,
				Rev: v.Rev,
				Ops: v.Ops.Clone(),
			})
		case im.Write:
			fd, ok := c.getFd(v.Doc)
			if !ok {
				panic("conn got WRITE with bad doc")
			}
			c.ws.WriteJSON(msg.Msg{
				Cmd: msg.C_WRITE,
				Fd:  fd,
				Rev: v.Rev,
				Ops: v.Ops.Clone(),
			})
		}
	}
}
