## Overview

`focus` is an experimental collaboration platform.

Its first major component, `vaporpad`, is a low-latency collaborative editor
inspired by and derived from [etherpad-lite](http://etherpad.org),
[sharejs](http://sharejs.org), and [ot.v](https://github.com/Operational-
Transformation/ot.v).

## Caveats

Warning: `focus` is not yet feature-complete and has [known
issues](https://github.com/mstone/focus/issues).

## Dependencies

focus:

  * build-depends on

      * [Golang](http://golang.org),
      * [sqlite3](http://sqlite.org),

    and several MIT- and Apache 2.0-licensed Golang libraries including

      * [golang/glog](https://github.com/golang/glog),
      * [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3),
      * [gopherjs](https://github.com/gopherjs/gopherjs),
      * [go-angularjs](https://github.com/gopherjs/go-angularjs),
      * [martini](https://github.com/go-martini/martini),
      * [martini-contrib/binding](https://github.com/martini-contrib/binding),
      * [martini-contrib/render](https://github.com/martini-contrib/render),
      * [gorilla/websocket](https://github.com/gorilla/websocket)
      * [codegangsta/inject](https://github.com/codegangsta/inject)
      * [phaikawl/options](https://github.com/phaikawl/options)

  * run-depends on

      * [AngularJS](https://angularjs.org), and

  * bundles:

      * [ACE](http://ace.c9.io)

## Use

The `focus` source repository is intended to be mounted in your GOPATH,
presently at `$GOPATH/src/akamai/focus`.

For ideas on how to run a `focus` instance, please see our example
[run.sh](./run.sh) script.
