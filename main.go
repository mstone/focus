// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package main runs the Focus server.
package main

import (
	"database/sql"
	"flag"
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
		Store: store,
		API:   *api,
	}

	server := server.New(serverCfg)

	err = server.Run()
	if err != nil {
		glog.Fatalf("unable to run server, err: %q", err)
	}

}
