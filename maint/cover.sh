#!/bin/bash -eu
go test -covermode=set -coverpkg=github.com/mstone/focus/ot,github.com/mstone/focus/internal/server,github.com/mstone/focus/internal/connection,github.com/mstone/focus/internal/document -c github.com/mstone/focus/internal/server

./server.test -test.coverprofile=cover.ok -test.run=Random &> focus.ok

go tool cover -html=cover.ok
