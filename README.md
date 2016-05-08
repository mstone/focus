## Overview

focus is an experimental collaboration platform.

Its first major component, `vaporpad` ([design doc](./docs/intent.adoc)), is a
low-latency collaborative editor inspired by and derived from
[etherpad-lite](http://etherpad.org), [sharejs](http://sharejs.org), and
[ot.v](https://github.com/Operational- Transformation/ot.v).

## Caveats

Warning: focus is not yet feature-complete and has [known
issues](https://github.com/mstone/focus/issues) that make it pre-alpha quality.

## Dependency Management

focus depends on [nix](https://nixos.org/nix/) and
[nixpkgs](https://github.com/NixOS/nixpkgs) for fine-grain dependency
management.

## Use

After installing nix, focus can be built by running:

```bash
git clone --recursive https://github.com/mstone/focus
cd focus
make
```

## Development

After installing nix, focus can be developed by running:

```bash
git clone --recursive https://github.com/mstone/focus
cd focus
make dev
```

to enter a development shell, and then by running

```bash
go generate
go build -i
go build
```

and so on.

## Docker

Finally, to use Docker to pack the resulting executable together with any resources necessary for deployment, run

```
make docker
docker load < result
docker run -v $(pwd)/data:/data -p 127.0.0.1:3000:3000 focus /bin/focus -api=ws://localhost:3000/ws -bind=0.0.0.0:3000 -log=/data/focus.log -dsn=/data/focus.db
```
