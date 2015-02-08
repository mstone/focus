// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package server provides an HTTP interface to the Focus store.
package server

import (
	"net/http"
	"path"
	"runtime/debug"
	"time"

	"github.com/unrolled/render"

	log "gopkg.in/inconshreveable/log15.v2"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/websocket"
	"github.com/mstone/focus/internal/server"
	"github.com/mstone/focus/store"
)

type WSConn struct {
	*websocket.Conn
}

func (w WSConn) SetReadTimeout(d time.Duration) error {
	return w.SetReadDeadline(time.Now().Add(d))
}
func (w WSConn) SetWriteTimeout(d time.Duration) error {
	return w.SetWriteDeadline(time.Now().Add(d))
}
func (w WSConn) CancelReadTimeout() error {
	return w.SetReadDeadline(time.Time{})
}
func (w WSConn) CancelWriteTimeout() error {
	return w.SetWriteDeadline(time.Time{})
}

type Config struct {
	API    string
	Assets string
	Store  *store.Store
}

type Server struct {
	m      *negroni.Negroni
	l      log.Logger
	s      *server.Server
	store  *store.Store
	api    string
	assets string
}

func New(c Config, is *server.Server) (*Server, error) {
	s := &Server{
		l:      log.Root(),
		s:      is,
		store:  c.Store,
		api:    c.API,
		assets: c.Assets,
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

		ws2 := WSConn{ws}

		c, err := s.s.AllocConn(ws2)
		if err != nil {
			log.Error("server unable to allocate new conn no", "err", err)
			return
		}

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

	go s.s.Run()

	return nil
}
