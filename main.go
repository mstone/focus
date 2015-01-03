// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package main runs the Focus server.
package main

import (
	"database/sql"
	"flag"
	"go/build"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"runtime/debug"

	_ "github.com/mattn/go-sqlite3"
	log "gopkg.in/inconshreveable/log15.v2"

	"github.com/mstone/focus/server"
	"github.com/mstone/focus/store"
)

var driver = flag.String("driver", "sqlite3", "database/sql driver")
var dsn = flag.String("dsn", ":memory:", "database/sql dsn")
var api = flag.String("api", "ws://localhost:3000/ws", "API endpoint")
var assets = flag.String("assets", defaultAssetPath(), "assets directory")
var logPath = flag.String("log", "./focus.log", "log path")

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

	fh := log.Must.FileHandler(*logPath, log.LogfmtFormat())
	// h := log.CallerStackHandler("%v",
	h := log.CallerFileHandler(
		log.MultiHandler(
			fh,
			log.StdoutHandler,
		),
	)
	log.Root().SetHandler(h)

	defer func() {
		r := recover()
		if r != nil {
			log.Error("focus caught panic", "debugstack", debug.Stack())
		}
		fh.(io.Closer).Close()
		if r != nil {
			os.Exit(1)
		}
	}()

	log.Info("focus", "boot", true)

	db, err := sql.Open(*driver, *dsn)
	if err != nil {
		log.Crit("unable to open driver", "driver", *driver, "dsn", *dsn, "err", err)
		return
	}

	storeCfg := store.Config{
		DB: db,
	}

	store := store.New(storeCfg)

	err = store.Reset()
	if err != nil {
		log.Crit("unable to reset store", "err", err)
		return
	}

	serverCfg := server.Config{
		Store:  store,
		API:    *api,
		Assets: *assets,
	}

	server, err := server.New(serverCfg)
	if err != nil {
		log.Crit("unable to configure server", "err", err)
		return
	}

	apiUrl, err := url.Parse(*api)
	if err != nil {
		log.Crit("unable to parse API URL", "err", err)
		return
	}

	log.Info("focus server starting", "host", apiUrl.Host)
	err = http.ListenAndServe(apiUrl.Host, server)
	if err != nil {
		log.Crit("unable to run server", "err", err)
		return
	}
}
