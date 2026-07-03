NAME    := valhall
GOOS    ?= linux
GOARCH  ?= $(shell go env GOARCH)
VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || git log -1 --format=%h 2>/dev/null || echo dev)
LDFLAGS := -X 'git.pynezz.dev/pynezz/stoker/cmd/stoker.version=$(VERSION)'
BINARY  := dist/$(NAME)

.PHONY: all build clean test vet check install help

all: build

build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
	  -ldflags "$(LDFLAGS)" \
	  -o $(BINARY) ./cmd/stoker

test:
	go test ./...

vet:
	go vet ./...

check: vet
	go run ./cmd/stoker --check

clean:
	rm -f $(BINARY)

install: build
	install -m 755 $(BINARY) /usr/local/bin/$(NAME)

help:
	@printf 'Usage: make [target]\n\n'
	@printf '  build              compile → $(BINARY)  (GOOS=$(GOOS) GOARCH=$(GOARCH))\n'
	@printf '  GOARCH=arm64 make  cross-compile example\n'
	@printf '  test               go test ./...\n'
	@printf '  vet                go vet ./...\n'
	@printf '  check              headless module + plugin load (CI gate)\n'
	@printf '  install            install $(BINARY) → /usr/local/bin/$(NAME)\n'
	@printf '  clean              remove $(BINARY)\n'
	@printf '\n  version: $(VERSION)\n'
