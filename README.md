## Overview

focus is an experimental collaboration platform.

Its first major component, `vaporpad` ([design doc](./docs/intent.adoc)), is a
low-latency collaborative editor inspired by and derived from
[etherpad-lite](http://etherpad.org), [sharejs](http://sharejs.org), and
[ot.v](https://github.com/Operational- Transformation/ot.v).

## Caveats

Warning: focus is not yet feature-complete and has [known
issues](https://github.com/mstone/focus/issues).

## Dependencies

focus:

  * build-depends on

      * [Golang 1.4](http://golang.org),
      * [sqlite3](http://sqlite.org),

    and several MIT-, 3BSD- and Apache 2.0-licensed Golang libraries including

      * [gopkg.in/inconshreveable/log15.v2](https://gopkg.in/inconshreveable/log15.v2),
      * [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3),
      * [mattn/go-colorable](https://github.com/mattn/go-colorable),
      * [gopherjs](https://github.com/gopherjs/gopherjs),
      * [codegangsta/negroni](https://github.com/codegangsta/negroni),
      * [gorilla/websocket](https://github.com/gorilla/websocket),

  * bundles:

      * [ACE](http://ace.c9.io)

## Use

Here are some hints to help get you started running a local dev instance of focus:

```bash
export GOPATH=$HOME/go
export PATH=$GOPATH/bin:$PATH
mkdir -p $GOPATH/{pkg,src,bin}

go get -u github.com/tools/godep
go get -u github.com/jteeuwen/go-bindata/...
go get -u github.com/mjibson/esc

go get -d github.com/mstone/focus
cd $GOPATH/src/github.com/mstone/focus

git submodule init
git submodule update
(cd public/gopherjs; godep go install)

godep restore
go generate
go build -i
go build
```

Also, for deployment, you might try adapting something like

```
make
docker build .
docker run -v $(pwd)/focus.log:/focus.log -p 127.0.0.1:3000:3000 $IMG "-api=ws://127.0.0.1:3000/" "-bind=0.0.0.0:3000"
```

to suit your network settings (or you might otherwise enjoy the resulting statically linked executable.)