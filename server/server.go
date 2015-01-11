// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package server provides an HTTP interface to the Focus store.
package server

import (
	"fmt"
	"github.com/unrolled/render"
	"net/http"
	"path"
	"runtime/debug"
	"sync"

	log "gopkg.in/inconshreveable/log15.v2"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/websocket"
	"github.com/mstone/focus/ot"
	"github.com/mstone/focus/store"
)

type Config struct {
	API    string
	Assets string
	Store  *store.Store
}

type Server struct {
	m        *negroni.Negroni
	l        log.Logger
	msgs     chan interface{}
	store    *store.Store
	api      string
	assets   string
	conns    map[*conn]struct{}
	names    map[string]*doc
	nextFd   int
	nextConn int
}

func New(c Config) (*Server, error) {
	s := &Server{
		msgs:     make(chan interface{}),
		l:        log.Root(),
		store:    c.Store,
		api:      c.API,
		assets:   c.Assets,
		conns:    map[*conn]struct{}{},
		names:    map[string]*doc{},
		nextFd:   0,
		nextConn: 0,
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
	s.conns[c] = struct{}{}
}

func (s *Server) openDoc(w chan allocdocresp, name string) {
	s.l.Info("server got allocdoc", "name", name)
	d, ok := s.names[name]
	if !ok {
		d = &doc{
			msgs:  make(chan interface{}),
			srvr:  s.msgs,
			wg:    sync.WaitGroup{},
			name:  name,
			conns: map[int]chan interface{}{},
			hist:  []ot.Ops{},
			comp:  ot.Ops{},
		}
		d.l = log.New(
			"obj", "doc",
			"doc", log.Lazy{d.String},
		)
		s.names[name] = d
		go d.Run()
	}
	s.l.Info("server sending allocdocresp", "doc", d)
	w <- allocdocresp{
		err: nil,
		doc: d.msgs,
	}
	s.l.Info("server finished allocdoc", "name", name)
}

/*

Proto:

cl ----  HELLO --->  srv
cl <---  *conn ----  srv
cl ----  OPEN ---->  conn
         name
                     conn  ----- allocdoc ----->  srv
                     conn  <----   *doc  -------  srv
                     conn  -----   open    ---->  doc
                                                  doc ----- allocfd -----> srv
                                                  doc <----  *fd   ------- srv
                     conn  <------  fd  --------  doc
cl <----  fd  -----  conn

*/

// processed by Server for conn
type allocdoc struct {
	reply chan allocdocresp
	name  string
}

type allocdocresp struct {
	err error
	doc chan interface{}
}

// processed by doc for conn
type open struct {
	dbgConn *conn
	conn    chan interface{}
	name    string
}

func (o open) String() string {
	return fmt.Sprintf("open{conn: %s, name: %s}", o.dbgConn, o.name)
}

type openresp struct {
	err     error
	dbgConn *conn
	doc     chan interface{}
	name    string
	fd      int
}

func (o openresp) String() string {
	errstr := "nil"
	if o.err != nil {
		errstr = o.err.Error()
	}
	return fmt.Sprintf("openresp{conn: %s, doc: <>, name: %s, fd: %d, err: %s}", o.dbgConn, o.name, o.fd, errstr)
}

// processed by Server for doc
type allocfd struct {
	reply chan allocfdresp
}

type allocfdresp struct {
	err error
	fd  int
}

// processed by Server for server
type allocconn struct {
	reply chan allocconnresp
}

type allocconnresp struct {
	err error
	no  int
}

type writeresp struct {
	dbgConn *conn
	fd      int
	rev     int
}

func (w writeresp) String() string {
	return fmt.Sprintf("writeresp{conn: %s, fd: %d, rev: %d}", w.dbgConn, w.fd, w.rev)
}

type write struct {
	dbgConn *conn
	fd      int
	rev     int
	ops     ot.Ops
}

func (w write) String() string {
	return fmt.Sprintf("write{conn: %s, fd: %d, rev: %d, ops: %s}", w.dbgConn, w.fd, w.rev, w.ops)
}

func (s *Server) allocFd(reply chan allocfdresp) {
	fd := s.nextFd
	s.nextFd++
	s.l.Info("server allocating fd", "fd", fd)
	reply <- allocfdresp{
		err: nil,
		fd:  fd,
	}
	s.l.Info("server sent allocfdresp", "fd", fd)
}

func (s *Server) allocConn(reply chan allocconnresp) {
	no := s.nextConn
	s.nextConn++
	s.l.Info("server allocating conn", "conn", no)
	reply <- allocconnresp{
		err: nil,
		no:  no,
	}
	s.l.Info("server sent allocconnresp", "conn", no)
}

func (s *Server) readLoop() {
	for m := range s.msgs {
		s.l.Info("server read msg", "cmd", m)
		switch v := m.(type) {
		default:
			s.l.Error("server got unknown msg", "cmd", m)
		case allocdoc:
			s.openDoc(v.reply, v.name)
		case allocfd:
			s.allocFd(v.reply)
		case allocconn:
			s.allocConn(v.reply)
		}
	}
}

func (s *Server) configure() error {
	m := negroni.New()
	m.Use(negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		log.Info("server http response starting", "obj", "server", "method", r.Method, "path", r.URL.Path, "hdrs", r.Header)
		next(w, r)
		log.Info("server http response finished", "obj", "server", "method", r.Method, "path", r.URL.Path, "hdrs", r.Header)
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
		log.Info("server starting websocket", "obj", "server", "remoteaddr", r.RemoteAddr, "hdrs", r.Header)
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error("server unable to upgrade incoming websocket connection", "err", err)
			return
		}

		srvrReplyChan := make(chan allocconnresp)
		s.msgs <- allocconn{srvrReplyChan}
		srvrResp := <-srvrReplyChan
		if srvrResp.err != nil {
			log.Error("server unable to allocate new conn no", "err", err)
			return
		}

		c := &conn{
			msgs:    make(chan interface{}),
			no:      srvrResp.no,
			numSend: 0,
			numRecv: 0,
			wg:      sync.WaitGroup{},
			ws:      ws,
			docs:    map[int]chan interface{}{},
			srvr:    s.msgs,
		}
		c.l = log.New(
			"obj", "conn",
			"conn", log.Lazy{c.String},
			// "numSend", log.Lazy{func() int { return c.numSend }},
			// "numRecv", log.Lazy{func() int { return c.numRecv }},
			// "total", log.Lazy{func() int { return c.numSend + c.numRecv }},
		)

		c.Run()
		log.Info("server finished websocket", "obj", "server", "remoteaddr", r.RemoteAddr, "hdrs", r.Header)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Info("server offering pad", "obj", "server", "pad", r.URL.Path)
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

	go s.readLoop()

	return nil
}
