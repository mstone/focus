#!/bin/bash -eu
go test -covermode=set -coverpkg=github.com/mstone/focus/ot,github.com/mstone/focus/internal/server -c github.com/mstone/focus/internal/server

until ! (./server.test -test.coverprofile=cover.err -test.run=Random &> focus.err); do
  echo test;
done

until (./server.test -test.coverprofile=cover.ok -test.run=Random &> focus.ok); do 
  echo test2;
done 

diff <(go tool cover -func=cover.ok) <(go tool cover -func=cover.err)
