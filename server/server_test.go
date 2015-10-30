// Copyright 2015 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package server

import (
	"go/build"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	log "gopkg.in/inconshreveable/log15.v2"

	"github.com/gorilla/websocket"

	"github.com/mstone/focus/internal/server"
	"github.com/mstone/focus/msg"
	"github.com/mstone/focus/ot"
	"github.com/mstone/focus/store"
)

func newTestServer(t *testing.T) (*httptest.Server, *Server) {
	pkg, err := build.Import("github.com/mstone/focus", "", build.FindOnly)
	if err != nil {
		t.Errorf("unable to locate server assets, err: %q", err)
	}

	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("unable to open test db, err: %q", err)
	}

	focusStore := store.New(db)

	err = focusStore.Reset()
	if err != nil {
		t.Fatalf("unable to reset store, err: %q", err)
	}

	focusConf := Config{
		Store:  focusStore,
		API:    "",
		Assets: http.Dir(pkg.Dir),
		Templates: func(assetPath string) ([]byte, error) {
			return ioutil.ReadFile(path.Join(pkg.Dir, "templates", assetPath))
		},
	}
	log.Info("test found assets path", "assets", focusConf.Assets)

	vppSrv, err := server.New(focusStore.Msgs())
	if err != nil {
		t.Errorf("error configuring INTERNAL focus test server; err: %q", err)
	}

	focusSrv, err := New(focusConf, vppSrv)
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

	wsconn, _, err := dialer.Dial(focusSrv.api, nil)
	if err != nil {
		t.Errorf("test unable to dial, err: %q", err)
	}
	defer wsconn.Close()

	conn := WSConn{wsconn}

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

func init() {
	log.Root().SetHandler(
		// log.CallerFileHandler(
		log.StderrHandler,
		// ),
	)
}
