// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package main runs the Focus server.
package main

import (
	"database/sql"
	"flag"
	"go/build"
	"net/http"
	"net/url"
	"runtime"
	"time"

	"github.com/golang/glog"
	_ "github.com/mattn/go-sqlite3"

	"github.com/mstone/focus/server"
	"github.com/mstone/focus/store"
)

var driver = flag.String("driver", "sqlite3", "database/sql driver")
var dsn = flag.String("dsn", ":memory:", "database/sql dsn")
var api = flag.String("api", "ws://localhost:3000/ws", "API endpoint")
var assets = flag.String("assets", defaultAssetPath(), "assets directory")

func defaultAssetPath() string {
	p, err := build.Default.Import("github.com/mstone/focus", "", build.FindOnly)
	if err != nil {
		return "."
	}
	return p.Dir
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.Parse()

	defer glog.Flush()

	go func() {
		for {
			glog.Flush()
			time.Sleep(time.Second)
		}
	}()

	db, err := sql.Open(*driver, *dsn)
	if err != nil {
		glog.Fatalf("unable to open driver: %q, dsn: %q, err: %q", *driver, *dsn, err)
	}

	storeCfg := store.Config{
		DB: db,
	}

	store := store.New(storeCfg)

	err = store.Reset()
	if err != nil {
		glog.Fatalf("unable to reset store, err: %q", err)
	}

	serverCfg := server.Config{
		Store:  store,
		API:    *api,
		Assets: *assets,
	}

	server, err := server.New(serverCfg)
	if err != nil {
		glog.Fatalf("unable to configure server, err: %q", err)
	}

	apiUrl, err := url.Parse(*api)
	if err != nil {
		glog.Fatalf("unable to parse API URL, err: %q", err)
	}

	err = http.ListenAndServe(apiUrl.Host, server)
	if err != nil {
		glog.Fatalf("unable to run server, err: %q", err)
	}

}
