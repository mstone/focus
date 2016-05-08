# NOTE: for information on how to manage dependencies, please see docs/HACKING.adoc

NIXPKGS_URL ?= https://github.com/NixOS/nixpkgs/archive/69420c52423f603083f5136430bfaa501c8b1c65.tar.gz
NIXPKGS_HASH ?= 0c2fdq1mzzbj0avgi31g5kb3rpzslaymnybwqbnys31ja0nz5lij
NIXPKGS_STOREPATH := $(shell env PRINT_PATH=1 nix-prefetch-url $(NIXPKGS_URL) $(NIXPKGS_HASH) 2>/dev/null | (read IGNORE; read STORE_PATH; echo $$STORE_PATH))

all:
	nix-build -j4 -I nixpkgs=file://$(NIXPKGS_STOREPATH) -A bin shell.nix

dev:
	nix-shell -j4 -I nixpkgs=file://$(NIXPKGS_STOREPATH)

docker:
	nix-build -j4 -I nixpkgs=file://$(NIXPKGS_STOREPATH) docker.nix

.PHONY: all dev docker
.DEFAULT_GOAL := all
