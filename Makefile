# NOTE: for information on how to manage dependencies, please see docs/HACKING.adoc

NCPU ?= $(shell nproc || sysctl -n hw.ncpu)
NCPU := $(NCPU)

all:
	nix-build --cores $(NCPU) -j$(NCPU) -A bin shell.nix

dev:
	nix-shell --cores $(NCPU) -j$(NCPU) -A bin

docker:
	nix-build --cores $(NCPU) -j$(NCPU) -A bin docker.nix

.PHONY: all dev docker
.DEFAULT_GOAL := all
