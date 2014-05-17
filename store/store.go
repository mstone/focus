// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package store persists Focus messages.
package store

import (
	"database/sql"
	"fmt"

	"github.com/golang/glog"
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
		glog.Errorf("unable to begin txn; err: %q", err)
		return
	}
	defer func() {
		if p := recover(); p != nil {
			switch p := p.(type) {
			case error:
				glog.Errorf("txn panic; err: %q", p)
				err = p
			default:
				glog.Errorf("txn panic; type: %t, err: %q", p, p)
				err = fmt.Errorf("%s", p)
			}
		}
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
		if err != nil {
			glog.Errorf("txn err on commit; err: %q", err)
			tx.Rollback()
			return
		}
	}()
	return txFunc(tx)
}

func transact2(db *sql.DB, txFunc func(*sql.Tx) (interface{}, error)) (ret interface{}, err error) {
	tx, err := db.Begin()
	if err != nil {
		glog.Errorf("unable to begin txn; err: %q", err)
		return
	}
	defer func() {
		if p := recover(); p != nil {
			switch p := p.(type) {
			case error:
				glog.Errorf("txn panic; err: %q", p)
				err = p
			default:
				glog.Errorf("txn panic; type: %t, err: %q", p, p)
				err = fmt.Errorf("%s", p)
			}
		}
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
		if err != nil {
			glog.Errorf("txn err on commit; err: %q", err)
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
