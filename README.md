## Overview

focus is an experimental collaboration platform.

Its first major component, `vaporpad` ([design doc](./docs/intent.adoc)), is a
low-latency collaborative editor inspired by and derived from
[etherpad-lite](http://etherpad.org), [sharejs](http://sharejs.org), and
[ot.v](https://github.com/Operational- Transformation/ot.v).

## Caveats

Warning: focus is not yet feature-complete and has [known
issues](https://github.com/mstone/focus/issues) that make it pre-alpha quality.

## Use

For the brave, here are some hints to help get you started running a local dev instance of focus:

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

Then, when you're ready to think about deployment, use:

```
make
```

to produce a statically linked executable.

Finally, to use Docker to pack the resulting executable together with any resources necessary for deployment, try adapting something like the following to suit your network configuration:

```
docker build .
docker run -v $(pwd)/focus.log:/focus.log -p 127.0.0.1:3000:3000 $IMG "-api=ws://my.site:3000/" "-bind=0.0.0.0:3000"
```
