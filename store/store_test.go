// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package store

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	log "gopkg.in/inconshreveable/log15.v2"
)

func mkTestStore(t *testing.T) *Store {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("unable to open test db, err: %q", err)
	}

	conf := Config{
		DB: db,
	}

	store := New(conf)

	err = store.Reset()
	if err != nil {
		t.Fatalf("unable to reset test db, err: %q", err)
	}

	return store
}

func TestStore(t *testing.T) {
	t.Parallel()

	s := mkTestStore(t)
	if s == nil {
		t.Fatalf("mkTestStore failed")
	}

	// empty
	log.Info("store test begin")
}
