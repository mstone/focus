package server

import (
	"github.com/gorilla/websocket"
	"time"
	// "flag"
	"net/http/httptest"
	"testing"

	"github.com/golang/glog"
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

	t.Logf("test: %p, got new testing api addr: %s", t, api)

	return httpSrv, focusSrv
}

func TestServer(t *testing.T) {
	_, focusSrv := newTestServer(t)

	dialer := websocket.Dialer{}

	conn, _, err := dialer.Dial(focusSrv.api, nil)
	if err != nil {
		t.Errorf("unable to dial, err: %q", err)
	}
	defer conn.Close()
}

func init() {
	// flag.Parse()

	defer glog.Flush()

	go func() {
		for {
			glog.Flush()
			time.Sleep(time.Second)
		}
	}()
}
