package server

import (
	"fmt"
	"go/build"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	log "gopkg.in/inconshreveable/log15.v2"

	"github.com/google/gofuzz"
	"github.com/gorilla/websocket"

	"github.com/mstone/focus/msg"
	"github.com/mstone/focus/ot"
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
	log.Info("test found assets path", "assets", focusConf.Assets)

	focusSrv, err := New(focusConf)
	if err != nil {
		t.Errorf("error configuring focus test server; err: %q", err)
	}

	httpSrv := httptest.NewServer(focusSrv)

	api := "ws://" + httpSrv.Listener.Addr().String() + "/ws"

	focusSrv.api = api

	log.Info("test got new testing api addr", "api", api)

	return httpSrv, focusSrv
}

func TestGetPad(t *testing.T) {
	httpSrv, _ := newTestServer(t)

	resp, err := http.Get(httpSrv.URL)
	if err != nil {
		t.Errorf("test unable to GET /; err: %q", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("test GET / did not return 200, resp: %#v, body: %s", resp, string(body))
	}
}

func TestAPI(t *testing.T) {
	_, focusSrv := newTestServer(t)

	dialer := websocket.Dialer{}

	conn, _, err := dialer.Dial(focusSrv.api, nil)
	if err != nil {
		t.Errorf("test unable to dial, err: %q", err)
	}
	defer conn.Close()

	// BUG(mistone): OPEN / really should probably fail, though we'll test that it works today.
	vpName := "/"
	log.Info("test opening vp", "name", vpName)
	conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
	err = conn.WriteJSON(msg.Msg{
		Cmd:  msg.C_OPEN,
		Name: vpName,
	})
	if err != nil {
		t.Errorf("unable to write OPEN, err: %q", err)
	}

	log.Info("test awaiting OPEN_RESP", "name", vpName)
	// read open resp
	m := msg.Msg{}
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	err = conn.ReadJSON(&m)
	if err != nil {
		t.Fatalf("test unable to read OPEN_RESP, err: %q", err)
	}

	if m.Cmd != msg.C_OPEN_RESP {
		t.Fatalf("test did not get an OPEN_RESP; msg: %+v", m)
	}

	if m.Name != vpName {
		t.Fatalf("server opened a different vaporpad: %s vs %+v", vpName, m)
	}

	fd := m.Fd
	log.Info("test received OPEN_RESP", "name", vpName, "fd", fd)

	// read initial write
	m = msg.Msg{}
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	err = conn.ReadJSON(&m)
	if err != nil {
		t.Errorf("test unable to read initial WRITE, err: %q", err)
	}

	if m.Cmd != msg.C_WRITE {
		t.Error("test did not get a WRITE; msg: %+v", m)
	}

	if m.Fd != fd {
		t.Errorf("test received WRITE for a different vaporpad: fd %d vs %+v", fd, m)
	}

	log.Info("test sending empty ops", "name", vpName, "fd", fd)
	// send empty ops
	conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
	err = conn.WriteJSON(msg.Msg{
		Cmd: msg.C_WRITE,
		Fd:  m.Fd,
		Rev: m.Rev,
		Ops: ot.Ops{},
	})
	if err != nil {
		t.Errorf("test unable to send WRITE, err: %q", err)
	}

	// read ack
	m = msg.Msg{}
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	err = conn.ReadJSON(&m)
	if err != nil {
		t.Errorf("test unable to read WRITE_RESP, err: %q", err)
	}

	if m.Cmd != msg.C_WRITE_RESP {
		t.Error("test did not get a WRITE_RESP; msg: %+v", m)
	}

	if m.Fd != fd {
		t.Errorf("test received a WRITE_RESP for a different vaporpad: fd %d vs %+v", fd, m)
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
	plen int
	l    log.Logger
}

func (c *client) sendRandomOps() {
	c.mu.Lock()
	defer c.mu.Unlock()

	defer func() {
		err := recover()
		if err != nil {
			c.l.Error("client caught panic", "err", err, "debugstack", debug.Stack())
		}
	}()

	c.l.Info("client sending random ops", "name", c.name, "fd", c.fd)

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
	c.l.Info("client generated ops", "ops", ops)

	c.doc.Apply(ops)
	c.st = c.st.Client(c, ops)

	c.l.Info("client send returned")
}

func (c *client) Send(ops ot.Ops) {
	c.plen++
	//c.ws.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
	err := c.ws.WriteJSON(msg.Msg{
		Cmd: msg.C_WRITE,
		Fd:  c.fd,
		Rev: c.rev,
		Ops: ops,
	})
	if err != nil {
		c.l.Error("client unable to send WRITE", "err", err)
	}
}

func (c *client) String() string {
	return fmt.Sprintf("{%p, %d}", c, c.plen)
}

func (c *client) Recv(rev int, ops ot.Ops) {
	c.l.Info("client recv", "rev", rev, "ops", ops)
	pdoc := c.doc.String()
	c.doc.Apply(ops)
	ndoc := c.doc.String()
	c.l.Info("client recv done", "fd", c.fd, "prev", pdoc, "next", ndoc)
}

func (c *client) Ack(rev int) {
	c.rev = rev
}

func (c *client) onWriteResp(m msg.Msg) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.l.Info("client got WRITE_RESP")
	c.st = c.st.Ack(c, m.Rev)
}

func (c *client) onWrite(m msg.Msg) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.l.Info("client got WRITE")
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
			c.l.Error("client unable to read response", "err", err)
		}
		c.plen++
		c.l.Info("client read msg", "client", c, "msg", m)
		switch m.Cmd {
		case msg.C_WRITE_RESP:
			c.onWriteResp(m)
		case msg.C_WRITE:
			c.onWrite(m)
		}
	}
}

const numClients = 2
const numRounds = 3

func TestRandom(t *testing.T) {
	go func() {
		time.Sleep(300 * time.Millisecond)
		panic("boom")
	}()

	_, focusSrv := newTestServer(t)

	wg := &sync.WaitGroup{}

	clients := make([]*client, numClients)

	run := func(idx int) {
		defer wg.Done()

		// BUG(mistone): OPEN / really should probably fail, though we'll test that it works today.
		vpName := "/"

		cwg := &sync.WaitGroup{}

		c := &client{
			mu:   sync.Mutex{},
			wg:   cwg,
			name: vpName,
			rev:  0,
			doc:  ot.NewDoc(),
			st:   &ot.Synchronized{},
		}
		clients[idx] = c
		c.l = log.New("client", log.Lazy{func() string {
			return c.String()
		}})

		dialer := websocket.Dialer{}

		c.l.Info("client dialing server", "api", focusSrv.api)

		conn, _, err := dialer.Dial(focusSrv.api, nil)
		if err != nil {
			t.Errorf("unable to dial, err: %q", err)
		}
		defer conn.Close()

		c.l.Info("client websocket open", "api", focusSrv.api)
		c.ws = conn

		c.l.Info("client sending OPEN", "name", vpName)
		conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		err = conn.WriteJSON(msg.Msg{
			Cmd:  msg.C_OPEN,
			Name: vpName,
		})
		if err != nil {
			t.Errorf("unable to write OPEN, err: %q", err)
		}

		c.l.Info("client awaiting OPEN_RESP", "name", vpName)
		// read open resp
		m := msg.Msg{}
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		err = conn.ReadJSON(&m)
		if err != nil {
			t.Errorf("server unable to read OPEN_RESP, err: %q", err)
		}
		conn.SetReadDeadline(time.Time{})

		if m.Cmd != msg.C_OPEN_RESP {
			t.Errorf("client did not get an OPEN_RESP; msg: %+v", m)
		}

		if m.Name != vpName {
			t.Errorf("client got OPEN_RESP with wrong vaporpad: %s vs %+v", vpName, m)
		}
		c.name = vpName
		c.fd = m.Fd
		c.l.Info("client got OPEN_RESP", "name", c.name, "fd", c.fd)

		c.l.Info("client starting reading + writing", "name", c.name, "fd", c.fd)
		cwg.Add(2)
		go c.writeLoop()
		go c.readLoop()
		cwg.Wait()
		c.l.Info("client done")
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
	log.Root().SetHandler(
		log.CallerFileHandler(
			log.StderrHandler,
		),
	)
}
