// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package server provides an HTTP interface to the Focus store.
package server

import (
	"net/http"
	"sync"

	"github.com/go-martini/martini"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/martini-contrib/render"

	"github.com/mstone/focus/msg"
	"github.com/mstone/focus/ot"
	"github.com/mstone/focus/store"
)

type Config struct {
	Store *store.Store
}

// struct conn represents an open WebSocket connection.
type conn struct {
	mu    sync.Mutex
	msgs  chan interface{}
	ws    *websocket.Conn
	descs map[*desc]struct{}
}

// struct desc represents an open VPP pad description (like an fd)
type desc struct {
	conn *conn
	doc  *doc
}

// struct doc represents a vaporpad (like a file)
type doc struct {
	mu    sync.Mutex
	descs map[*desc]struct{}
	hist  ot.Ops
}

type Server struct {
	mu    sync.Mutex
	store *store.Store
	conns map[*conn]struct{}
	descs map[*desc]struct{}
	docs  map[*doc]struct{}
}

func New(c Config) *Server {
	s := &Server{
		mu:    sync.Mutex{},
		store: c.Store,
		conns: map[*conn]struct{}{},
		descs: map[*desc]struct{}{},
		docs:  map[*doc]struct{}{},
	}

	doc := &doc{
		mu:    sync.Mutex{},
		descs: map[*desc]struct{}{},
		hist:  ot.Ops{},
	}

	s.docs[doc] = struct{}{}

	return s
}

func jsonError(x render.Render, status int, v interface{}) {
	x.JSON(status, v)
}

func (s *Server) addConn(c *conn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	var doc *doc
	for d2, _ := range s.docs {
		doc = d2
	}

	doc.mu.Lock()
	defer doc.mu.Unlock()

	d := &desc{
		conn: c,
		doc:  doc,
	}

	s.conns[c] = struct{}{}
	c.descs[d] = struct{}{}
	s.descs[d] = struct{}{}
	doc.descs[d] = struct{}{}
}

func (s *Server) closeConn(c *conn) {
	// XXX: this code is likely buggy: tightly coupled, badly locked, and buggy! :-/

	s.mu.Lock()
	defer s.mu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.ws.Close()
	close(c.msgs)

	unlink := func(desc *desc) {
		desc.doc.mu.Lock()
		defer desc.doc.mu.Unlock()

		delete(desc.doc.descs, desc)
		delete(s.descs, desc)
		delete(c.descs, desc)
	}

	for desc, _ := range c.descs {
		unlink(desc)
	}

	delete(s.conns, c)
}

func (s *Server) transformOps(c *conn, rev int, ops ot.Ops) {
	s.mu.Lock()
	defer s.mu.Unlock()

	glog.Infof("conn: %p, transforming %d ops", c, len(ops))

	// extract last desc
	c.mu.Lock()
	defer c.mu.Unlock()

	var d *desc
	for d2, _ := range c.descs {
		d = d2
	}

	// extract doc
	doc := d.doc

	// process ops
	doc.mu.Lock()
	defer doc.mu.Unlock()

	var concurrent ot.Ops
	if rev < len(doc.hist) {
		concurrent = doc.hist[rev:]
	}
	glog.Infof("conn: %p, desc: %p, doc: %p, found %d concurrent ops", c, d, doc, len(concurrent))

	// go func() {
	// 	time.Sleep(1 * time.Second)
	// 	panic("boom")
	// }()
	ops2, concurrent2 := ot.Transform(ops, concurrent)

	glog.Infof("transform:\n\tops: %s -> ops2: %s\n\tcon: %s -> con2: %s", ops.String(), ops2.String(), concurrent.String(), concurrent2.String())

	doc.hist = append(doc.hist, ops2...)
	rev = len(doc.hist)

	glog.Infof("conn: %p, enqueueing", c)

	send := func(pdesc *desc) {
		if pdesc == d {
			pdesc.conn.msgs <- ack{rev}
		} else {
			pdesc.conn.mu.Lock()
			defer pdesc.conn.mu.Unlock()

			pdesc.conn.msgs <- write{rev, ops2}
		}
	}

	for pdesc, _ := range doc.descs {
		send(pdesc)
	}
}

func (s *Server) readConn(c *conn) {
	// XXX: need to properly lock c + detect channel closure...
	for {
		var m msg.OTClientMsg

		if err := c.ws.ReadJSON(&m); err != nil {
			glog.Errorf("reading ops; got err %q", err)
			return
		}

		glog.Infof("conn: %p, read acks: %d, ops: %s", c, m.Rev, m.Ops)

		s.transformOps(c, m.Rev, m.Ops)

		glog.Infof("conn: %p, done enqueueing", c)
	}
}

type ack struct {
	rev int
}

type write struct {
	rev int
	ops ot.Ops
}

func (s *Server) Run() error {
	m := martini.Classic()

	m.Use(render.Renderer(render.Options{
		Delims: render.Delims{
			Left:  "((",
			Right: "))",
		},
	}))

	m.Get("/", func(x render.Render) {
		x.HTML(200, "root", nil)
	})

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	m.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			glog.Errorf("unable to upgrade incoming websocket connection, err: %q", err)
			return
		}

		c := &conn{
			mu:    sync.Mutex{},
			msgs:  make(chan interface{}),
			ws:    ws,
			descs: map[*desc]struct{}{},
		}

		defer s.closeConn(c)

		s.addConn(c)

		go s.readConn(c)

		for m := range c.msgs {
			glog.Infof("conn: %p: msg: %#v", c, m)
			switch v := m.(type) {
			case ack:
				c.ws.WriteJSON(msg.OTServerMsg{
					Rev: v.rev,
					Ack: true,
					Ops: nil,
				})
			case write:
				c.ws.WriteJSON(msg.OTServerMsg{
					Rev: v.rev,
					Ack: false,
					Ops: v.ops,
				})
			}
		}

		glog.Infof("conn %p: exiting", c)
	})

	m.Run()

	return nil
}
