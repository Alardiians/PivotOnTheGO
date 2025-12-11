BIN ?= pivotonthego
PREFIX ?= $(HOME)/.local/bin

.PHONY: build install run clean

build:
	go build -o bin/$(BIN) ./cmd/ui

install:
	mkdir -p $(PREFIX)
	go build -o $(PREFIX)/$(BIN) ./cmd/ui

run:
	go run ./cmd/ui

clean:
	rm -f bin/$(BIN)
