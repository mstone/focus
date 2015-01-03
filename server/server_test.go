package server

import (
	"flag"
	"github.com/google/gofuzz"
	"github.com/mstone/focus/msg"
	"github.com/mstone/focus/ot"
	"go/build"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/websocket"
)

func newTestServer(t *testing.T) (*httptest.Server, *Server) {
	pkg, err := build.Import("github.com/mstone/focus", "", build.FindOnly)
	if err != nil {
		t.Errorf("unable to locate server assets, err: %q", err)
	}

	focusConf := Config{
		Store:  nil,
		API:    "",
		Assets: pkg.Dir,
	}
	glog.Infof("found assets path: %q", focusConf.Assets)

	focusSrv, err := New(focusConf)
	if err != nil {
		t.Errorf("error configuring focus test server; err: %q", err)
	}

	httpSrv := httptest.NewServer(focusSrv)

	api := "ws://" + httpSrv.Listener.Addr().String() + "/ws"

	focusSrv.api = api

	glog.Infof("test: %p, got new testing api addr: %s", t, api)

	return httpSrv, focusSrv
}

func TestGetPad(t *testing.T) {
	httpSrv, _ := newTestServer(t)

	resp, err := http.Get(httpSrv.URL)
	if err != nil {
		t.Errorf("unable to GET /; err: %q", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("GET / did not return 200, resp: %#v, body: %s", resp, string(body))
	}
}

func TestAPI(t *testing.T) {
	_, focusSrv := newTestServer(t)

	dialer := websocket.Dialer{}

	conn, _, err := dialer.Dial(focusSrv.api, nil)
	if err != nil {
		t.Errorf("unable to dial, err: %q", err)
	}
	defer conn.Close()

	// BUG(mistone): OPEN / really should probably fail, though we'll test that it works today.
	vpName := "/"
	glog.Infof("opening vp %s", vpName)
	conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
	err = conn.WriteJSON(msg.Msg{
		Cmd:  msg.C_OPEN,
		Name: vpName,
	})
	if err != nil {
		t.Errorf("unable to write OPEN, err: %q", err)
	}

	glog.Infof("awaiting OPEN_RESP for %s", vpName)
	// read open resp
	m := msg.Msg{}
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	err = conn.ReadJSON(&m)
	if err != nil {
		t.Errorf("unable to read OPEN_RESP, err: %q", err)
	}

	if m.Cmd != msg.C_OPEN_RESP {
		t.Errorf("did not get an OPEN_RESP; msg: %+v", m)
	}

	if m.Name != vpName {
		t.Errorf("server opened a different vaporpad: %s vs %+v", vpName, m)
	}

	fd := m.Fd
	glog.Infof("OPEN_RESP for %s yielded fd %d", vpName, fd)

	// read initial write
	m = msg.Msg{}
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	err = conn.ReadJSON(&m)
	if err != nil {
		t.Errorf("unable to read initial WRITE, err: %q", err)
	}

	if m.Cmd != msg.C_WRITE {
		t.Error("did not get a WRITE; msg: %+v", m)
	}

	if m.Fd != fd {
		t.Errorf("server sent WRITE for a different vaporpad: fd %d vs %+v", fd, m)
	}

	glog.Infof("sending empty ops for %s/%d", vpName, fd)
	// send empty ops
	conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
	err = conn.WriteJSON(msg.Msg{
		Cmd: msg.C_WRITE,
		Fd:  m.Fd,
		Rev: m.Rev,
		Ops: ot.Ops{},
	})
	if err != nil {
		t.Errorf("unable to send WRITE, err: %q", err)
	}

	// read ack
	m = msg.Msg{}
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	err = conn.ReadJSON(&m)
	if err != nil {
		t.Errorf("unable to read WRITE_RESP, err: %q", err)
	}

	if m.Cmd != msg.C_WRITE_RESP {
		t.Error("did not get a WRITE_RESP; msg: %+v", m)
	}

	if m.Fd != fd {
		t.Errorf("server sent WRITE_RESP for a different vaporpad: fd %d vs %+v", fd, m)
	}
}

type client struct {
	mu   sync.Mutex
	wg   *sync.WaitGroup
	name string
	fd   int
	ws   *websocket.Conn
	rev  int
	doc  *ot.Doc
	st   ot.State
}

func (c *client) sendRandomOps() {
	c.mu.Lock()
	defer c.mu.Unlock()

	defer func() {
		err := recover()
		if err != nil {
			glog.Errorf("client %p, caught panic: %q, stack: %s", err, debug.Stack())
		}
	}()

	glog.Infof("sending random ops for %s/%d", c.name, c.fd)

	ops := ot.Ops{}
	f := fuzz.New().NilChance(0).Funcs(
		func(p *ot.Ops, fc fuzz.Continue) {
			size := c.doc.Len()
			op := 0
			if size > 0 {
				op = fc.Intn(2)
			}
			switch op {
			case 0:
				s := fc.RandString()
				pos := 0
				if size > 0 {
					pos = fc.Intn(size)
				}
				*p = ot.NewInsert(size, pos, s)
			case 1:
				d := fc.Intn(size - 1)
				pos := 0
				if size > 0 {
					pos = fc.Intn(size - d)
				}
				*p = ot.NewDelete(size, pos, d)
			}
		},
	)
	f.NumElements(1, 1).Fuzz(&ops)
	glog.Infof("client: %p, sending random ops: %s", c, ops.String())

	c.doc.Apply(ops)
	c.st = c.st.Client(c, ops)
}

func (c *client) Send(ops ot.Ops) {
	//c.ws.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
	err := c.ws.WriteJSON(msg.Msg{
		Cmd: msg.C_WRITE,
		Fd:  c.fd,
		Rev: c.rev,
		Ops: ops,
	})
	if err != nil {
		glog.Errorf("unable to send WRITE, err: %q", err)
	}
}

func (c *client) Recv(rev int, ops ot.Ops) {
	glog.Infof("client: %p, fd: %d, doc: %#v, ops: %s", c, c.fd, c.doc, ops)
	pdoc := c.doc.String()
	c.doc.Apply(ops)
	ndoc := c.doc.String()
	glog.Infof("client: %p, fd: %d:\n\tpdoc: %q\n\tndoc: %q", c, c.fd, pdoc, ndoc)
}

func (c *client) Ack(rev int) {
	c.rev = rev
}

func (c *client) onWriteResp(m msg.Msg) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.st = c.st.Ack(c, m.Rev)
}

func (c *client) onWrite(m msg.Msg) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.st = c.st.Server(c, m.Rev, m.Ops)
}

func (c *client) writeLoop() {
	defer c.wg.Done()

	for i := 0; i < numRounds; i++ {
		c.sendRandomOps()
	}
}

func (c *client) readLoop() {
	defer c.wg.Done()

	for i := 0; i < numRounds*numClients+1; i++ {
		m := msg.Msg{}
		//c.ws.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		err := c.ws.ReadJSON(&m)
		if err != nil {
			glog.Errorf("client %p: unable to read response, err: %q", c, err)
		}
		switch m.Cmd {
		case msg.C_WRITE_RESP:
			c.onWriteResp(m)
		case msg.C_WRITE:
			c.onWrite(m)
		}
	}
}

const numClients = 2
const numRounds = 1

func TestRandom(t *testing.T) {
	_, focusSrv := newTestServer(t)

	wg := &sync.WaitGroup{}

	clients := make([]client, numClients)

	run := func(idx int) {
		defer wg.Done()

		dialer := websocket.Dialer{}

		conn, _, err := dialer.Dial(focusSrv.api, nil)
		if err != nil {
			t.Errorf("unable to dial, err: %q", err)
		}
		defer conn.Close()

		// BUG(mistone): OPEN / really should probably fail, though we'll test that it works today.
		vpName := "/"
		glog.Infof("opening vp %s", vpName)
		conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		err = conn.WriteJSON(msg.Msg{
			Cmd:  msg.C_OPEN,
			Name: vpName,
		})
		if err != nil {
			t.Errorf("unable to write OPEN, err: %q", err)
		}

		glog.Infof("awaiting OPEN_RESP for %s", vpName)
		// read open resp
		m := msg.Msg{}
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		err = conn.ReadJSON(&m)
		if err != nil {
			t.Errorf("unable to read OPEN_RESP, err: %q", err)
		}
		conn.SetReadDeadline(time.Time{})

		if m.Cmd != msg.C_OPEN_RESP {
			t.Errorf("did not get an OPEN_RESP; msg: %+v", m)
		}

		if m.Name != vpName {
			t.Errorf("server opened a different vaporpad: %s vs %+v", vpName, m)
		}

		fd := m.Fd
		glog.Infof("OPEN_RESP for %s yielded fd %d", vpName, fd)

		cwg := &sync.WaitGroup{}

		c := client{
			mu:   sync.Mutex{},
			wg:   cwg,
			name: vpName,
			fd:   fd,
			ws:   conn,
			rev:  0,
			doc:  ot.NewDoc(),
			st:   &ot.Synchronized{},
		}
		clients[idx] = c

		cwg.Add(2)
		go c.writeLoop()
		go c.readLoop()
		cwg.Wait()
	}

	wg.Add(numClients)
	for i := 0; i < numClients; i++ {
		go run(i)
	}
	wg.Wait()

	for i := 1; i < numClients; i++ {
		s1 := clients[0].doc.String()
		s2 := clients[i].doc.String()
		if s1 != s2 {
			t.Errorf("error, doc[0] != doc[%d]\n\t%q\n\t%q", i, s1, s2)
		}
	}
}

func init() {
	flag.Parse()

	defer glog.Flush()

	go func() {
		for {
			glog.Flush()
			time.Sleep(time.Second)
		}
	}()
}
