package server

import (
	"flag"
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

	t.Logf("test: %p, got new testing api addr: %s", t, api)

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
	flag.Parse()

	defer glog.Flush()

	go func() {
		for {
			glog.Flush()
			time.Sleep(time.Second)
		}
	}()
}
