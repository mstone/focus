package server

import (
	"sync"

	log "gopkg.in/inconshreveable/log15.v2"

	"github.com/mstone/focus/ot"
)

type Server struct {
	l        log.Logger
	msgs     chan interface{}
	conns    map[*conn]struct{}
	names    map[string]*doc
	nextFd   int
	nextConn int
}

func New() (*Server, error) {
	s := &Server{
		msgs:     make(chan interface{}),
		l:        log.Root(),
		conns:    map[*conn]struct{}{},
		names:    map[string]*doc{},
		nextFd:   0,
		nextConn: 0,
	}
	return s, nil
}

func (s *Server) addConn(c *conn) {
	s.conns[c] = struct{}{}
}

func (s *Server) openDoc(w chan allocdocresp, name string) {
	d, ok := s.names[name]
	if !ok {
		d = &doc{
			msgs:  make(chan interface{}),
			srvr:  s.msgs,
			wg:    sync.WaitGroup{},
			name:  name,
			conns: map[int]dconn{},
			hist:  []ot.Ops{},
			comp:  ot.Ops{},
		}
		s.names[name] = d
		go d.Run()
	}
	w <- allocdocresp{
		err: nil,
		doc: d.msgs,
	}
}

func (s *Server) allocFd(reply chan allocfdresp) {
	fd := s.nextFd
	s.nextFd++
	reply <- allocfdresp{
		err: nil,
		fd:  fd,
	}
}

func (s *Server) allocConn(reply chan allocconnresp) {
	no := s.nextConn
	s.nextConn++
	reply <- allocconnresp{
		err: nil,
		no:  no,
	}
}

func (s *Server) AllocConn(ws WebSocket) (*conn, error) {
	srvrReplyChan := make(chan allocconnresp)
	s.msgs <- allocconn{srvrReplyChan}
	srvrResp := <-srvrReplyChan

	if srvrResp.err != nil {
		return nil, srvrResp.err
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

	return c, nil
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

func (s *Server) Run() {
	s.readLoop()
}
