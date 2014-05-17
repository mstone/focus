#!/bin/bash -eu
(mkdir -p public; cd public; gopherjs build ../client/client.go)
go run main.go -logtostderr=true "$@"
