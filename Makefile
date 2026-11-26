GO ?= go
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test install

build:
	$(GO) build -ldflags "-X main.version=$(VERSION)" -o sway-ws ./cmd/sway-ws

test:
	$(GO) test ./...

install: build
	install -D -m 0755 sway-ws $(HOME)/.local/bin/sway-ws
