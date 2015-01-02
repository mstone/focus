package server

import (
	"flag"
	"github.com/mstone/focus/msg"
	"github.com/mstone/focus/ot"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/websocket"
)

func newTestServer(t *testing.T) (*httptest.Server, *Server) {
	focusConf := Config{
		Store: nil,
		API:   "",
	}

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
