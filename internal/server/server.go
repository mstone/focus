// Copyright 2015 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package server

import (
	"github.com/mstone/focus/internal/connection"
	"github.com/mstone/focus/internal/document"
	im "github.com/mstone/focus/internal/msgs"
)

type Server struct {
	msgs     chan interface{}
	names    map[string]chan interface{}
	nextFd   int
	nextConn int
}

func New() (*Server, error) {
	s := &Server{
		msgs:     make(chan interface{}),
		names:    map[string]chan interface{}{},
		nextFd:   0,
		nextConn: 0,
	}
	go s.readLoop()
	return s, nil
}

func (s *Server) onAllocDoc(w chan im.Allocdocresp, name string) {
	var d chan interface{}
	var ok bool

	d, ok = s.names[name]
	if !ok {
		d = document.New(s.msgs, name)
		s.names[name] = d
	}
	w <- im.Allocdocresp{
		Err: nil,
		Doc: d,
	}
}

func (s *Server) onAllocFd(reply chan im.Allocfdresp) {
	fd := s.nextFd
	s.nextFd++
	reply <- im.Allocfdresp{
		Err: nil,
		Fd:  fd,
	}
}

func (s *Server) onAllocConn(reply chan im.Allocconnresp) {
	no := s.nextConn
	s.nextConn++
	reply <- im.Allocconnresp{
		Err: nil,
		No:  no,
	}
}

func (s *Server) Connect(ws connection.WebSocket) (chan interface{}, error) {
	srvrReplyChan := make(chan im.Allocconnresp)
	s.msgs <- im.Allocconn{srvrReplyChan}
	srvrResp := <-srvrReplyChan

	if srvrResp.Err != nil {
		return nil, srvrResp.Err
	}

	c := connection.New(s.msgs, ws)
	return c, nil
}

func (s *Server) readLoop() {
	for m := range s.msgs {
		switch v := m.(type) {
		default:
		case im.Allocdoc:
			s.onAllocDoc(v.Reply, v.Name)
		case im.Allocfd:
			s.onAllocFd(v.Reply)
		case im.Allocconn:
			s.onAllocConn(v.Reply)
		}
	}
}
