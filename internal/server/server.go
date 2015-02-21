package server

import (
	"github.com/mstone/focus/internal/connection"
	"github.com/mstone/focus/internal/document"
	im "github.com/mstone/focus/internal/msgs"
	log "gopkg.in/inconshreveable/log15.v2"
)

type Server struct {
	l        log.Logger
	msgs     chan interface{}
	names    map[string]chan interface{}
	nextFd   int
	nextConn int
}

func New() (*Server, error) {
	s := &Server{
		msgs:     make(chan interface{}),
		l:        log.Root(),
		names:    map[string]chan interface{}{},
		nextFd:   0,
		nextConn: 0,
	}
	return s, nil
}

func (s *Server) openDoc(w chan im.Allocdocresp, name string) {
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

func (s *Server) allocFd(reply chan im.Allocfdresp) {
	fd := s.nextFd
	s.nextFd++
	reply <- im.Allocfdresp{
		Err: nil,
		Fd:  fd,
	}
}

func (s *Server) allocConn(reply chan im.Allocconnresp) {
	no := s.nextConn
	s.nextConn++
	reply <- im.Allocconnresp{
		Err: nil,
		No:  no,
	}
}

func (s *Server) AllocConn(ws connection.WebSocket) (chan interface{}, error) {
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
			s.l.Error("server got unknown msg", "cmd", m)
		case im.Allocdoc:
			s.openDoc(v.Reply, v.Name)
		case im.Allocfd:
			s.allocFd(v.Reply)
		case im.Allocconn:
			s.allocConn(v.Reply)
		}
	}
}

func (s *Server) Run() {
	s.readLoop()
}
