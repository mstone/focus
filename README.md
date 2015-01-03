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

      * [Golang](http://golang.org),
      * [sqlite3](http://sqlite.org),

    and several MIT-, 3BSD- and Apache 2.0-licensed Golang libraries including

      * [golang/glog](https://github.com/golang/glog),
      * [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3),
      * [gopherjs](https://github.com/gopherjs/gopherjs) (now forked for go1.3 compatibility: [mstone/gopherjs](https://github.com/mstone/gopherjs)),
      * [martini](https://github.com/go-martini/martini),
      * [martini-contrib/binding](https://github.com/martini-contrib/binding),
      * [martini-contrib/render](https://github.com/martini-contrib/render),
      * [gorilla/websocket](https://github.com/gorilla/websocket)
      * [codegangsta/inject](https://github.com/codegangsta/inject)
      * [gofuzz](https://github.com/google/gofuzz)
      * [bpool](https://github.com/oxtoacart/bpool)

  * bundles:

      * [ACE](http://ace.c9.io)

## Use

Here are some hints to help get you started running a local dev instance of focus:

```bash
export GOPATH=$HOME/go
mkdir -p $GOPATH/{pkg,src,bin}

export PATH=$GOPATH/bin:$PATH

go get -u github.com/tools/godep

go get -d github.com/mstone/gopherjs
cd $GOPATH/src/github.com/mstone/gopherjs
git remote update
git checkout -b go1.3 origin/go1.3
godep go install

go get -d github.com/mstone/focus
cd $GOPATH/src/github.com/mstone/focus
git submodule init
git submodule update
godep restore
./run.sh
```

and here's a step-by-step explanation of these instructions:

1. focus is written in go and go expects you to set a special environment
variable, named `GOPATH`, to point to a special directory structure intended
for storing source code, intermediate compilation results, and compiled
binaries:

    ```bash
    export GOPATH=$HOME/go
    mkdir -p $GOPATH/{pkg,src,bin}
    ```

2. focus build-depends on some other programs written in go, notably
[godep](https://github.com/tools/godep) and
[gopherjs](https://github.com/gopherjs/gopherjs). Therefore, we add the
`$GOPATH/bin` folder to our `PATH` environment variable:

    ```bash
    export PATH=$GOPATH/bin:$PATH
    ```

3. gopherjs and focus are still under active development so, for now, we
recommend using [godep](https://github.com/tools/godep) to stay in sync with
our versions of the relevant build-deps:

    ```bash
    go get -u github.com/tools/godep
    ```

4. which we use to build gopherjs, as in:

    ```bash
    go get -d github.com/mstone/gopherjs
    cd $GOPATH/src/github.com/mstone/gopherjs
    git remote update
    git checkout -b go1.3 origin/go1.3
    godep go install
    ```

5. finally, focus uses some existing javascript libraries like
[ACE](http://ace.c9.io), which we currently include via a [git
submodule](http://git-scm.com/docs/git-submodule), which [go
get](http://golang.org/cmd/go/#hdr-Download_and_install_packages_and_dependencies)
does not automatically check out:

    ```bash
    go get -d github.com/mstone/focus
    cd $GOPATH/src/github.com/mstone/focus
    git submodule init
    git submodule update
    godep restore
    ./run.sh
    ```
