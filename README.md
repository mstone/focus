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

## Getting The Code

After installing nix, focus can be checked out by running:

```bash
git clone --recursive https://github.com/mstone/focus
cd focus
```

## Use

To build focus from a clean checkout, run:

```bash
make
```

## Development

To enter a focus dev-shell configured for interactive development, run:

```bash
make dev
```

Then edit and run commands like:

```bash
go generate
go build -i
go build
```

to build from your (potentially dirty) working tree.

## Docker

Want to deploy via docker? No problem, just run:

```
make docker
docker load < result
docker run -v $(pwd)/data:/data -p 127.0.0.1:3000:3000 focus /bin/focus -api=ws://localhost:3000/ws -bind=0.0.0.0:3000 -log=/data/focus.log -dsn=/data/focus.db
```

(and customize as needed with your particular deployment settings.)
