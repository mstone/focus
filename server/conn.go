package server

import (
	"fmt"
	"github.com/mstone/focus/msg"
	"reflect"
	"strings"
	"sync"

	log "gopkg.in/inconshreveable/log15.v2"

	"github.com/gorilla/websocket"
)

// struct conn represents an open WebSocket connection.
type conn struct {
	msgs chan interface{}
	l    log.Logger
	wg   sync.WaitGroup
	ws   *websocket.Conn
	docs map[int]chan interface{}
	srvr chan interface{}
}

func (c *conn) String() string {
	return fmt.Sprintf("%p", c)
}

func (c *conn) Run() {
	c.wg.Add(2)
	go c.readLoop()
	go c.writeLoop()
	c.wg.Wait()

	c.l.Info("conn finished")
}

func (c *conn) Close() error {
	return nil
}

func (c *conn) readLoop() {
	defer c.wg.Done()

	for {
		var m msg.Msg

		if err := c.ws.ReadJSON(&m); err != nil {
			c.l.Error("conn read error; closing conn", "err", err)
			c.Close() // BUG(mistone): errcheck?
			return
		}

		switch m.Cmd {
		default:
			c.l.Error("conn got unknown cmd; exiting", "msg", m)
			return
		case msg.C_OPEN:
			c.l.Info("conn got OPEN, sending allocdoc", "msg", m)
			srvrReplyChan := make(chan allocdocresp)
			c.srvr <- allocdoc{
				reply: srvrReplyChan,
				name:  m.Name,
			}

			srvrResp := <-srvrReplyChan
			if srvrResp.err != nil {
				c.l.Error("conn unable to allocdoc", "err", srvrResp.err)
				panic("conn unable to allocdoc")
			}

			c.l.Info("conn finished allocdoc, sending open", "msg", m)

			doc := srvrResp.doc
			doc <- open{
				conn: c.msgs,
			}
			c.l.Info("conn finished OPEN", "msg", m)
		case msg.C_WRITE:
			c.l.Info("conn got WRITE", "msg", m)
			doc, ok := c.docs[m.Fd]
			if !ok {
				c.l.Error("conn got WRITE with bad fd, exiting")
				panic("conn got WRITE with bad fd")
			}
			c.l.Info("conn enqueuing write for doc", "msg", m, "doc", doc)
			doc <- write{
				fd:  m.Fd,
				rev: m.Rev,
				ops: m.Ops,
			}
			c.l.Info("conn finished WRITE", "msg", m)
		}
	}
}

func (c *conn) writeLoop() {
	defer c.wg.Done()

	for m := range c.msgs {
		c.l.Info("conn read internal msg", "msgtype", reflect.TypeOf(m).Name(), "msg", m)
		c.l.Info("server writing "+strings.ToUpper(reflect.TypeOf(m).Name()), "msg", m)
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
	}
}
