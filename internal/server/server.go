// Copyright 2015 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package server

import (
	log "gopkg.in/inconshreveable/log15.v2"

	"github.com/mstone/focus/internal/connection"
	"github.com/mstone/focus/internal/document"
	im "github.com/mstone/focus/internal/msgs"
)

type Server struct {
	msgs  chan interface{}
	names map[string]chan interface{}
	store chan interface{}
}

func New(store chan interface{}) (*Server, error) {
	s := &Server{
		msgs:  make(chan interface{}),
		names: map[string]chan interface{}{},
		store: store,
	}
	go s.readLoop()
	return s, nil
}

func (s *Server) onAllocDoc(w chan im.Allocdocresp, name string) {
	var d chan interface{}
	var ok bool
	var err error

	d, ok = s.names[name]
	if !ok {
		d, err = document.New(s.msgs, s.store, name)
		if err != nil {
			log.Error("unable to create document", "name", name, "err", err)
			w <- im.Allocdocresp{
				Err: err,
				Doc: nil,
			}
			return
		}
		s.names[name] = d
	}
	w <- im.Allocdocresp{
		Err: nil,
		Doc: d,
	}
}

func (s *Server) Connect(ws connection.WebSocket) (chan interface{}, error) {
	c := connection.New(s.msgs, ws)
	return c, nil
}

func (s *Server) readLoop() {
	for m := range s.msgs {
		switch v := m.(type) {
		default:
		case im.Allocdoc:
			s.onAllocDoc(v.Reply, v.Name)
		}
	}
}
