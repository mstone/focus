// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package server provides an HTTP interface to the Focus store.
package server

import (
	"crypto/rand"
	"math/big"
	"net/http"
	"sync"

	"github.com/go-martini/martini"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/martini-contrib/render"

	"akamai/focus/msg"
	"akamai/focus/ot"
	"akamai/focus/store"
)

type Config struct {
	Store *store.Store
}

type otconn struct {
	site  int
	conn  *websocket.Conn
	cmsgs chan interface{}
}

type Server struct {
	mu    sync.Mutex
	store *store.Store

	conns map[int]otconn
	hist  ot.Ops
}

func New(c Config) *Server {
	return &Server{
		mu:    sync.Mutex{},
		store: c.Store,
		conns: map[int]otconn{},
		hist:  ot.Ops{},
	}
}

func jsonError(x render.Render, status int, v interface{}) {
	x.JSON(status, v)
}

func (s *Server) addConn(site int, conn *websocket.Conn) otconn {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conns[site] = otconn{
		site:  site,
		conn:  conn,
		cmsgs: make(chan interface{}, 5),
	}
	return s.conns[site]
}

func (s *Server) closeConn(site int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn := s.conns[site]
	conn.conn.Close()
	close(conn.cmsgs)
	delete(s.conns, site)
}

func (s *Server) transformOps(site int, rev int, ops ot.Ops) {
	s.mu.Lock()
	defer s.mu.Unlock()

	glog.Infof("site: %d, transforming %d ops", site, len(ops))

	var concurrent ot.Ops
	if rev < len(s.hist) {
		concurrent = s.hist[rev:]
	}
	glog.Infof("site: %d, found %d concurrent ops", site, len(concurrent))

	// go func() {
	// 	time.Sleep(1 * time.Second)
	// 	panic("boom")
	// }()
	ops2, concurrent2 := ot.Transform(ops, concurrent)

	glog.Infof("transform:\n\tops: %s -> ops2: %s\n\tcon: %s -> con2: %s", ops.String(), ops2.String(), concurrent.String(), concurrent2.String())

	s.hist = append(s.hist, ops2...)
	rev = len(s.hist)

	glog.Infof("site: %d, enqueueing", site)

	for peersite, peerconn := range s.conns {
		if site == peersite {
			peerconn.cmsgs <- ack{rev}
		} else {
			peerconn.cmsgs <- write{site, rev, ops2}
		}
	}
}

func (s *Server) readConn(conn otconn) {
	for {
		var m msg.OTClientMsg

		if err := conn.conn.ReadJSON(&m); err != nil {
			glog.Errorf("reading ops; got err %q", err)
			return
		}

		glog.Infof("site: %d, read acks: %d, ops: %s", conn.site, m.Rev, m.Ops)

		s.transformOps(conn.site, m.Rev, m.Ops)

		glog.Infof("site: %d, done enqueueing", conn.site)
	}
}

type ack struct {
	rev int
}

type write struct {
	site int
	rev  int
	ops  ot.Ops
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
		siteBig, err := rand.Int(rand.Reader, big.NewInt(1e9))
		if err != nil {
			glog.Errorf("unable to generate random site id, err: %q", err)
			return
		}
		site := int(siteBig.Int64())

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			glog.Errorf("unable to upgrade incoming websocket connection, err: %q", err)
			return
		}
		defer s.closeConn(site)
		otc := s.addConn(site, conn)

		go s.readConn(otc)

		for cmsg := range otc.cmsgs {
			glog.Infof("site: %d: cmsg: %#v", site, cmsg)
			switch v := cmsg.(type) {
			case ack:
				otc.conn.WriteJSON(msg.OTServerMsg{
					Site: site,
					Rev:  v.rev,
					Ack:  true,
					Ops:  nil,
				})
			case write:
				otc.conn.WriteJSON(msg.OTServerMsg{
					Site: v.site,
					Rev:  v.rev,
					Ack:  false,
					Ops:  v.ops,
				})
			}
		}

		glog.Infof("site %d: exiting", site)
	})

	m.Get("/register_site.json", func(x render.Render) {
		site, err := rand.Int(rand.Reader, big.NewInt(1e9))
		if err != nil {
			glog.Errorf("unable to generate random site id, err: %q", err)
			jsonError(x, http.StatusInternalServerError, "")
			return
		}

		glog.Infof("registering site")

		x.JSON(200, site.Int64())
	})

	m.Run()

	return nil
}
