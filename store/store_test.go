// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package store

import (
	"database/sql"
	"testing"

	"github.com/golang/glog"
	_ "github.com/mattn/go-sqlite3"
)

func mkTestStore(t *testing.T) *Store {
	db, err := sql.Open("sqlite3", "../fizzle.db")
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
	defer glog.Flush()
	t.Parallel()

	s := mkTestStore(t)
	if s == nil {
		t.Fatalf("mkTestStore failed")
	}

	// empty
	glog.Infof("begin")
}
