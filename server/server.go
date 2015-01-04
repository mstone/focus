// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package server provides an HTTP interface to the Focus store.
package server

import (
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
	m      *negroni.Negroni
	l      log.Logger
	msgs   chan interface{}
	store  *store.Store
	api    string
	assets string
	conns  map[*conn]struct{}
	docs   map[*doc]struct{}
	names  map[string]*doc
	next   int
}

func New(c Config) (*Server, error) {
	s := &Server{
		msgs:   make(chan interface{}),
		l:      log.Root(),
		store:  c.Store,
		api:    c.API,
		assets: c.Assets,
		conns:  map[*conn]struct{}{},
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
		d.l = log.New("doc", log.Lazy{
			func() string {
				return d.String()
			},
		})
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
	conn chan interface{}
	name string
}

type openresp struct {
	err  error
	doc  chan interface{}
	name string
	fd   int
}

// processed by Server for doc
type allocfd struct {
	reply chan allocfdresp
}

type allocfdresp struct {
	err error
	fd  int
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

func (s *Server) allocFd(reply chan allocfdresp) {
	fd := s.next
	s.next++
	s.l.Info("server allocating fd", "fd", fd)
	reply <- allocfdresp{
		err: nil,
		fd:  fd,
	}
	s.l.Info("server sent allocfdresp", "fd", fd)
}

func (s *Server) readLoop() {
	for m := range s.msgs {
		s.l.Error("server read msg", "msg", m)
		switch v := m.(type) {
		default:
			s.l.Error("server got unknown msg", "msg", m)
		case allocdoc:
			s.openDoc(v.reply, v.name)
		case allocfd:
			s.allocFd(v.reply)
		}
	}
}

func (s *Server) configure() error {
	m := negroni.New()
	m.Use(negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		log.Info("server http response starting", "method", r.Method, "path", r.URL.Path, "hdrs", r.Header)
		next(w, r)
		log.Info("server http response finished", "method", r.Method, "path", r.URL.Path, "hdrs", r.Header)
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
			msgs: make(chan interface{}),
			wg:   sync.WaitGroup{},
			ws:   ws,
			docs: map[int]chan interface{}{},
			srvr: s.msgs,
		}
		c.l = log.New("conn", log.Lazy{
			func() string {
				return c.String()
			},
		})

		c.Run()
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

	go s.readLoop()

	return nil
}
