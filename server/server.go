// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package server provides an HTTP interface to the Focus store.
package server

import (
	"go/build"
	"net/http"
	"path"
	"reflect"
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
	API   string
	Store *store.Store
}

// struct conn represents an open WebSocket connection.
type conn struct {
	mu    sync.Mutex
	msgs  chan interface{}
	ws    *websocket.Conn
	descs map[int]*desc
}

// struct desc represents an open VPP pad description (like an fd)
type desc struct {
	no   int
	conn *conn
	doc  *doc
}

// struct doc represents a vaporpad (like a file)
type doc struct {
	mu    sync.Mutex
	descs map[*desc]struct{}
	hist  []ot.Ops
	comp  ot.Ops
}

type Server struct {
	m     *martini.ClassicMartini
	mu    sync.Mutex
	store *store.Store
	api   string
	conns map[*conn]struct{}
	descs map[*desc]struct{}
	docs  map[*doc]struct{}
	names map[string]*doc
	next  int
}

func New(c Config) (*Server, error) {
	s := &Server{
		mu:    sync.Mutex{},
		store: c.Store,
		api:   c.API,
		conns: map[*conn]struct{}{},
		descs: map[*desc]struct{}{},
		docs:  map[*doc]struct{}{},
		names: map[string]*doc{},
		next:  1,
	}

	err := s.configure()
	if err != nil {
		glog.Errorf("unable to configure server, err: %q", err)
		return nil, err
	}

	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.m.ServeHTTP(w, r)
}

func jsonError(x render.Render, status int, v interface{}) {
	x.JSON(status, v)
}

func (s *Server) addConn(c *conn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conns[c] = struct{}{}
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
		delete(c.descs, desc.no)
	}

	for _, desc := range c.descs {
		unlink(desc)
	}

	delete(s.conns, c)
}

func (s *Server) transformOps(c *conn, fd int, rev int, ops ot.Ops) {
	s.mu.Lock()
	defer s.mu.Unlock()

	glog.Infof("conn: %p, transforming %d ops", c, len(ops))

	// extract last desc
	c.mu.Lock()
	defer c.mu.Unlock()

	d, ok := c.descs[fd]
	if !ok {
		glog.Errorf("conn: %p, fd: %d, rev: %d: invalid fd", c, fd, rev)
	}

	// extract doc
	doc := d.doc

	// process ops
	doc.mu.Lock()
	defer doc.mu.Unlock()

	// extract concurrent ops
	cops := []ot.Ops{}
	if rev < len(doc.hist) {
		cops = doc.hist[rev:]
	}
	glog.Infof("conn: %p, desc: %p, doc: %p, found %d concurrent ops-lists", c, d, doc, len(cops))

	// transform input ops
	// tops := ops
	// for _, pop := range pops {
	// 	topsPrev := tops
	// 	//tops, _ = ot.Transform(tops, pop)
	// 	tops, _ = ot.Transform(tops, pop)
	// 	glog.Infof("transform:\n\tops: %s -> ops2: %s\n\tcon: %s", topsPrev.String(), tops.String(), pop.String())
	// }

	cops2 := ot.Ops{}
	for _, cop := range cops {
		cops2 = ot.Compose(cops2, cop)
	}
	tops, _ := ot.Transform(ops, cops2)

	tops2 := ops
	for _, cop := range cops {
		tops2, _ = ot.Transform(tops2, cop)
	}
	glog.Infof("xfrm:\n\tcops : %s\n\tcops2: %s\n\ttops : %s\n\ttops2: %s", cops, cops2, tops, tops2)

	hist := doc.hist
	comp := doc.comp
	hist2 := append(doc.hist, tops)
	comp2 := ot.Compose(doc.comp, tops)
	comp3 := ot.Compose(doc.comp, tops2)

	if !reflect.DeepEqual(ot.Normalize(comp2), ot.Normalize(comp3)) {
		glog.Errorf("compose <> transform!")
	}

	glog.Infof("doc:\n\thist : %s\n\thist2: %s\n\tcomp : %s\n\tops  : %s\n\ttops : %s\n\tcomp2: %s\n\tcomp3: %s", hist, hist2, comp, ops, tops, comp2, comp3)
	doc.hist = hist2
	doc.comp = comp2
	rev = len(doc.hist)

	send := func(pdesc *desc) {
		if pdesc == d {
			glog.Infof("conn: %p, pconn: %p, enqueueing %#v", c, pdesc.conn, writeresp{pdesc.no, rev})
			pdesc.conn.msgs <- writeresp{pdesc.no, rev}
		} else {
			pdesc.conn.mu.Lock()
			defer pdesc.conn.mu.Unlock()

			glog.Infof("conn: %p, pconn: %p, enqueueing server.write{fd: %d, rev: %d, ops: %s}", c, pdesc.conn, pdesc.no, rev, tops)
			pdesc.conn.msgs <- write{pdesc.no, rev, tops}
		}
	}

	for pdesc, _ := range doc.descs {
		send(pdesc)
	}
}

func (s *Server) openDoc(c *conn, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.names[name]
	if !ok {
		d = &doc{
			mu:    sync.Mutex{},
			descs: map[*desc]struct{}{},
			hist:  []ot.Ops{},
			comp:  ot.Ops{},
		}
		s.names[name] = d
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	d.mu.Lock()
	defer d.mu.Unlock()

	fd := &desc{
		no:   s.next,
		conn: c,
		doc:  d,
	}
	s.next++

	c.descs[fd.no] = fd
	d.descs[fd] = struct{}{}

	c.msgs <- openresp{
		name: name,
		fd:   fd.no,
	}

	// if len(d.hist) > 0 {
	c.msgs <- write{
		fd:  fd.no,
		rev: len(d.hist),
		ops: d.comp,
	}
	// }
}

func (s *Server) readConn(c *conn) {
	// XXX: need to properly lock c + detect channel closure...
	for {
		var m msg.Msg

		if err := c.ws.ReadJSON(&m); err != nil {
			glog.Errorf("reading ops; got err %q", err)
			return
		}

		switch m.Cmd {
		default:
			glog.Errorf("conn: %p, got unknown cmd: %q, exiting", c, m)
			s.closeConn(c)
			return
		case msg.C_OPEN:
			glog.Infof("conn: %p, got OPEN, name: %q", c, m.Name)
			s.openDoc(c, m.Name)
			glog.Infof("conn: %p, done opening name: %q", c, m.Name)
		case msg.C_WRITE:
			glog.Infof("conn: %p, got WRITE fd: %d, rev: %d, ops: %s", c, m.Fd, m.Rev, m.Ops)
			s.transformOps(c, m.Fd, m.Rev, m.Ops)
			glog.Infof("conn: %p, done with WRITE fd: %d, rev: %d, ops: %s", c, m.Fd, m.Rev, m.Ops)
		}
	}
}

func (s *Server) writeConn(c *conn) {
	for m := range c.msgs {
		glog.Infof("conn: %p: msg: %#v", c, m)
		switch v := m.(type) {
		case openresp:
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
			glog.Infof("conn: %p:\n\twriting fd: %d, rev: %d, ops: %s", c, v.fd, v.rev, v.ops)
			c.ws.WriteJSON(msg.Msg{
				Cmd: msg.C_WRITE,
				Fd:  v.fd,
				Rev: v.rev,
				Ops: v.ops,
			})
		}
	}
}

type open struct{}

type openresp struct {
	name string
	fd   int
}

type writeresp struct {
	fd  int
	rev int
}

type write struct {
	fd  int
	rev int
	ops ot.Ops
}

func (s *Server) defaultAssetPath() string {
	p, err := build.Default.Import("github.com/mstone/focus", "", build.FindOnly)
	if err != nil {
		return "."
	}
	return path.Join(p.Dir, "templates")
}
func (s *Server) configure() error {
	m := martini.Classic()

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	m.Use(render.Renderer(render.Options{
		Directory: s.defaultAssetPath(),
	}))

	m.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			glog.Errorf("unable to upgrade incoming websocket connection, err: %q", err)
			return
		}

		c := &conn{
			mu:    sync.Mutex{},
			msgs:  make(chan interface{}, 5),
			ws:    ws,
			descs: map[int]*desc{},
		}

		defer s.closeConn(c)

		s.addConn(c)

		go s.readConn(c)

		s.writeConn(c)

		glog.Infof("conn %p: exiting", c)
	})

	m.Get("/**", func(x render.Render, r *http.Request) {
		v := struct {
			API, Name string
		}{
			API:  s.api,
			Name: r.URL.Path,
		}
		x.HTML(200, "root", v)
	})

	s.m = m

	return nil
}
