// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package store persists Focus messages.
package store

import (
	"database/sql"
	"fmt"
	"runtime/debug"

	log "gopkg.in/inconshreveable/log15.v2"
)

type Config struct {
	DB *sql.DB
}

type Store struct {
	db *sql.DB
}

func New(config Config) *Store {
	return &Store{
		db: config.DB,
	}
}

// adapted from http://stackoverflow.com/questions/16184238/database-sql-tx-detecting-commit-or-rollback
func transact(db *sql.DB, txFunc func(*sql.Tx) error) (err error) {
	tx, err := db.Begin()
	if err != nil {
		log.Error("unable to begin txn", "err", err)
		return
	}
	defer func() {
		if p := recover(); p != nil {
			switch p := p.(type) {
			case error:
				log.Error("txn panic", "err", p, "debugstack", debug.Stack())
				err = p
			default:
				log.Error("txn panic", "err", p, "debugstack", debug.Stack())
				err = fmt.Errorf("%s", p)
			}
		}
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
		if err != nil {
			log.Error("txn err on commit", "err", err, "debugstack", debug.Stack())
			tx.Rollback()
			return
		}
	}()
	return txFunc(tx)
}

func transact2(db *sql.DB, txFunc func(*sql.Tx) (interface{}, error)) (ret interface{}, err error) {
	tx, err := db.Begin()
	if err != nil {
		log.Error("unable to begin txn", "err", err)
		return
	}
	defer func() {
		if p := recover(); p != nil {
			switch p := p.(type) {
			case error:
				log.Error("txn panic", "err", p, "debugstack", debug.Stack())
				err = p
			default:
				log.Error("txn panic", "err", p, "debugstack", debug.Stack())
				err = fmt.Errorf("%s", p)
			}
		}
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
		if err != nil {
			log.Error("txn err on commit", "err", err, "debugstack", debug.Stack())
			tx.Rollback()
			return
		}
	}()
	return txFunc(tx)
}

// Reset initializes the Store's db tables.
func (s *Store) Reset() error {
	return transact(s.db, func(tx *sql.Tx) error {
		return nil
	})
}
