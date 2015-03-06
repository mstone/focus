// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package main runs the Focus server.
package main

import (
	"database/sql"
	"flag"
	"io"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"

	_ "github.com/mattn/go-sqlite3"
	log "gopkg.in/inconshreveable/log15.v2"

	otserver "github.com/mstone/focus/internal/server"
	"github.com/mstone/focus/server"
	"github.com/mstone/focus/store"
)

//go:generate gopherjs build github.com/mstone/focus/client -m -o public/client.js
//go:generate esc -o assets.go -pkg=main -prefix=public/ public/client.js public/ace-builds/src-min-noconflict/
//go:generate go-bindata -o bindata.go -prefix=templates/ templates/

func main() {
	driver := ""
	dsn := ""
	api := ""
	bind := ""
	logPath := ""
	local := false

	flag.StringVar(&driver, "driver", "sqlite3", "database/sql driver")
	flag.StringVar(&dsn, "dsn", ":memory:", "database/sql dsn")
	flag.StringVar(&api, "api", "ws://localhost:3000/ws", "API endpoint")
	flag.StringVar(&bind, "bind", "localhost:3000", "ip:port to bind")
	flag.StringVar(&logPath, "log", "./focus.log", "log path")
	flag.BoolVar(&local, "local", false, "use local assets?")

	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.Parse()

	fh := log.Must.FileHandler(logPath, log.LogfmtFormat())
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

	db, err := sql.Open(driver, dsn)
	if err != nil {
		log.Crit("unable to open driver", "driver", driver, "dsn", dsn, "err", err)
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
		Store:     store,
		API:       api,
		Assets:    FS(local),
		Templates: Asset,
	}

	otServer, err := otserver.New()
	if err != nil {
		log.Crit("unable to configure ot-server", "err", err)
		return
	}

	server, err := server.New(serverCfg, otServer)
	if err != nil {
		log.Crit("unable to configure server", "err", err)
		return
	}

	log.Info("focus server starting", "bind", bind, "api", api)
	err = http.ListenAndServe(bind, server)
	if err != nil {
		log.Crit("unable to run server", "err", err)
		return
	}
}
