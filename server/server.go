// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package server provides an HTTP interface to the Focus store.
package server

import (
	"github.com/unrolled/render"
	"net/http"
	"path"
	"reflect"
	"runtime/debug"
	"sync"

	log "gopkg.in/inconshreveable/log15.v2"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/websocket"

	"github.com/mstone/focus/msg"
	"github.com/mstone/focus/ot"
	"github.com/mstone/focus/store"
)

type Config struct {
	API    string
	Assets string
	Store  *store.Store
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
	m      *negroni.Negroni
	mu     sync.Mutex
	store  *store.Store
	api    string
	assets string
	conns  map[*conn]struct{}
	descs  map[*desc]struct{}
	docs   map[*doc]struct{}
	names  map[string]*doc
	next   int
}

func New(c Config) (*Server, error) {
	s := &Server{
		mu:     sync.Mutex{},
		store:  c.Store,
		api:    c.API,
		assets: c.Assets,
		conns:  map[*conn]struct{}{},
		descs:  map[*desc]struct{}{},
		docs:   map[*doc]struct{}{},
		names:  map[string]*doc{},
		next:   1,
	}

	err := s.configure()
	if err != nil {
		log.Error("unable to configure server", "err", err)
		return nil, err
	}

	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.m.ServeHTTP(w, r)
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

	l := log.New("conn", c, "fd", fd, "rev", rev, "ops", ops)
	l.Info("server transforming ops")

	// extract last desc
	c.mu.Lock()
	defer c.mu.Unlock()

	d, ok := c.descs[fd]
	if !ok {
		l.Error("invalid fd")
	}

	// extract doc
	doc := d.doc

	// process ops
	doc.mu.Lock()
	defer doc.mu.Unlock()

	l = l.New("desc", d) //, "doc", doc)

	// extract concurrent ops
	cops := []ot.Ops{}
	if rev < len(doc.hist) {
		cops = doc.hist[rev:]
	}
	l.Info("server found concurrent ops-lists", "num", len(cops), "val", cops)

	cops2 := ot.Ops{}
	for _, cop := range cops {
		cops2 = ot.Compose(cops2, cop)
	}
	tops, _ := ot.Transform(ops, cops2)

	tops2 := ops
	for _, cop := range cops {
		tops2, _ = ot.Transform(tops2, cop)
	}
	l.Info("server xfrm", "cops", cops, "cops2", cops2, "tops", tops, "tops2", tops2)

	// hist := doc.hist
	// comp := doc.comp
	hist2 := append(doc.hist, tops)
	comp2 := ot.Compose(doc.comp, tops)
	comp3 := ot.Compose(doc.comp, tops2)

	if !reflect.DeepEqual(ot.Normalize(comp2), ot.Normalize(comp3)) {
		l.Error("compose <> transform!")
	}

	// l.Info("server result", "hist", hist, "hist2", hist2, "comp", comp, "comp2", comp2, "comp3", comp3, "tops", tops)
	doc.hist = hist2
	doc.comp = comp2
	rev = len(doc.hist)

	send := func(pdesc *desc) {
		l := l.New("pconn", pdesc.conn, "pfd", pdesc.no)
		if pdesc == d {
			l.Info("enqueueing writeresp", "msg", writeresp{pdesc.no, rev})
			pdesc.conn.msgs <- writeresp{pdesc.no, rev}
		} else {
			pdesc.conn.mu.Lock()
			defer pdesc.conn.mu.Unlock()

			l.Info("enqueueing write", "msg", write{pdesc.no, rev, tops})
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

	c.msgs <- write{
		fd:  fd.no,
		rev: len(d.hist),
		ops: d.comp,
	}
}

func (s *Server) readConn(c *conn) {
	// XXX: need to properly lock c + detect channel closure...
	l := log.New("conn", c)
	for {
		var m msg.Msg

		if err := c.ws.ReadJSON(&m); err != nil {
			l.Error("server reading ops", "err", err)
			return
		}

		switch m.Cmd {
		default:
			log.Error("server got unknown cmd; exiting", "msg", m)
			s.closeConn(c)
			return
		case msg.C_OPEN:
			log.Info("server got OPEN", "msg", m)
			s.openDoc(c, m.Name)
			log.Info("server finished OPEN", "msg", m)
		case msg.C_WRITE:
			log.Info("server got WRITE", "msg", m)
			s.transformOps(c, m.Fd, m.Rev, m.Ops)
			log.Info("server finished WRITE", "msg", m)
		}
	}
}

func (s *Server) writeConn(c *conn) {
	l := log.New("conn", c)
	for m := range c.msgs {
		l.Info("server writing", "msg", m)
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

func (s *Server) configure() error {
	m := negroni.New()
	m.Use(negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		log.Info("http request starting", "method", r.Method, "path", r.URL.Path, "hdrs", r.Header)
		next(w, r)
		log.Info("http request finished", "method", r.Method, "path", r.URL.Path, "hdrs", r.Header)
	}))
	m.Use(negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		defer func() {
			r := recover()
			if r != nil {
				log.Error("http caught panic", "debugstack", debug.Stack())
			}
		}()
		next(w, r)
	}))
	m.Use(negroni.NewStatic(http.Dir(path.Join(s.assets, "public"))))

	x := render.New(render.Options{
		Directory: path.Join(s.assets, "templates"),
	})

	mux := http.NewServeMux()

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		log.Info("server starting websocket", "remoteaddr", r.RemoteAddr, "hdrs", r.Header)
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error("server unable to upgrade incoming websocket connection", "err", err)
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

		log.Info("server finished conn", "conn", c)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Info("server offering pad", "pad", r.URL.Path)
		v := struct {
			API, Name string
		}{
			API:  s.api,
			Name: r.URL.Path,
		}
		x.HTML(w, 200, "root", v)
	})

	m.UseHandler(mux)

	s.m = m

	return nil
}
