// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package store persists Focus messages.
package store

import (
	"encoding/json"
	"fmt"
	"runtime/debug"

	"github.com/jmoiron/sqlx"
	log "gopkg.in/inconshreveable/log15.v2"

	im "github.com/mstone/focus/internal/msgs"
	"github.com/mstone/focus/ot"
)

type Store struct {
	msgs chan interface{}
	db   *sqlx.DB
}

func New(db *sqlx.DB) *Store {
	st := &Store{
		msgs: make(chan interface{}),
		db:   db,
	}
	go st.readLoop()
	return st
}

func (st *Store) Msgs() chan interface{} {
	return st.msgs
}

func (st *Store) readLoop() {
	for m := range st.msgs {
		switch v := m.(type) {
		default:
			log.Error("store got message with unknown type", "msg", m)
			panic(fmt.Errorf("store got message with unknown type, msg: %q", m))
		case im.Storedoc:
			st.onStoreDoc(v.Reply, v.Name)
		case im.Storewrite:
			st.onStoreWrite(v.Reply, v.DocId, v.Rev, v.Ops)
		}
	}
}

func (st *Store) onStoreDoc(reply chan im.Storedocresp, name string) {
	idBox, err := transact2(st.db, func(tx *sqlx.Tx) (interface{}, error) {
		res, err := tx.Exec("INSERT INTO document (id, name) VALUES (?, ?)", nil, name)
		if err != nil {
			log.Error("unable to insert store doc", "name", name, "err", err)
			return nil, err
		}
		return res.LastInsertId()
	})
	if err != nil {
		log.Error("unable to store doc", "name", name, "err", err)
		reply <- im.Storedocresp{Err: err}
		return
	}
	id := idBox.(int64)
	reply <- im.Storedocresp{
		Err:     nil,
		StoreId: id,
	}
}

func (st *Store) onStoreWrite(reply chan im.Storewriteresp, docId int64, rev int, ops ot.Ops) {
	idBox, err := transact2(st.db, func(tx *sqlx.Tx) (interface{}, error) {
		opsBytes, err := json.Marshal(ops)
		if err != nil {
			log.Error("unable to marshal ops", "ops", ops, "err", err)
			return nil, err
		}
		res, err := tx.Exec("INSERT INTO operation (id, document_id, author_id, revision_number, body) VALUES (?, ?, ?, ?, ?)", nil, docId, nil, rev, string(opsBytes))
		if err != nil {
			log.Error("unable to insert ops", "ops", ops, "err", err)
			return nil, err
		}
		return res.LastInsertId()
	})
	if err != nil {
		log.Error("unable to store ops", "err", err)
		reply <- im.Storewriteresp{Err: err}
		return
	}
	id := idBox.(int64)
	reply <- im.Storewriteresp{
		Err:     nil,
		StoreId: id,
	}
}

// adapted from http://stackoverflow.com/questions/16184238/database-sql-tx-detecting-commit-or-rollback
func transact(db *sqlx.DB, txFunc func(*sqlx.Tx) error) (err error) {
	tx, err := db.Beginx()
	if err != nil {
		log.Error("unable to begin txn", "err", err)
		return
	}
	defer func() {
		if p := recover(); p != nil {
			switch p := p.(type) {
			case error:
				log.Error("txn panic", "err", p, "debugstack", string(debug.Stack()))
				err = p
			default:
				log.Error("txn panic", "err", p, "debugstack", string(debug.Stack()))
				err = fmt.Errorf("%s", p)
			}
		}
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
		if err != nil {
			log.Error("txn err on commit", "err", err, "debugstack", string(debug.Stack()))
			tx.Rollback()
			return
		}
	}()
	return txFunc(tx)
}

func transact2(db *sqlx.DB, txFunc func(*sqlx.Tx) (interface{}, error)) (ret interface{}, err error) {
	tx, err := db.Beginx()
	if err != nil {
		log.Error("unable to begin txn", "err", err)
		return
	}
	defer func() {
		if p := recover(); p != nil {
			switch p := p.(type) {
			case error:
				log.Error("txn panic", "err", p, "debugstack", string(debug.Stack()))
				err = p
			default:
				log.Error("txn panic", "err", p, "debugstack", string(debug.Stack()))
				err = fmt.Errorf("%s", p)
			}
		}
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
		if err != nil {
			log.Error("txn err on commit", "err", err, "debugstack", string(debug.Stack()))
			tx.Rollback()
			return
		}
	}()
	return txFunc(tx)
}

// Reset initializes the Store's db tables.
func (s *Store) Reset() error {
	userVersionBox, err := transact2(s.db, func(tx *sqlx.Tx) (interface{}, error) {
		var userVersion int
		row := tx.QueryRow("PRAGMA user_version")
		err := row.Scan(&userVersion)
		if err != nil {
			log.Error("unable to scan user_version", "err", err)
			return 0, err
		}
		return userVersion, nil
	})
	if err != nil {
		return err
	}
	userVersion := userVersionBox.(int)

	log.Info("store found user_version", "user_version", userVersion)

	if userVersion < 1 {
		log.Info("store applying migration 1")
		transact(s.db, func(tx *sqlx.Tx) error {
			tx.MustExec(`CREATE TABLE IF NOT EXISTS document (
				id INTEGER PRIMARY KEY,
				name TEXT
				)`)
			tx.MustExec(`CREATE TABLE IF NOT EXISTS author (
				id INTEGER PRIMARY KEY,
				name TEXT,
				email TEXT
				)`)
			tx.MustExec(`CREATE TABLE IF NOT EXISTS operation (
				id INTEGER PRIMARY KEY,
				document_id INTEGER,
				author_id INTEGER,
				revision_number INTEGER,
				body TEXT,
				FOREIGN KEY (document_id) REFERENCES document(id),
				FOREIGN KEY (author_id) REFERENCES author(id)
				)`)
			tx.MustExec(`
				PRAGMA user_version = 1;
				`)
			return nil
		})
		log.Info("store finished migration 1")
	}
	return nil
}
