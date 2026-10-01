.PHONY: build test lint fmt install clean

VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/ambient .

test:
	go test ./...

lint:
	go vet ./...
	gofmt -l . | grep -q . && { echo "files need gofmt:"; gofmt -l .; exit 1; } || true

fmt:
	gofmt -w .

install:
	go install -ldflags "-X main.version=$(VERSION)" ./.

clean:
	rm -rf bin dist
