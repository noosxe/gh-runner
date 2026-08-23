# gh-runner — AIO Supervisor
#
# All targets are intended to run inside the Nix development shell:
#   nix develop --command make <target>

BINARY := supervisor
PKG     := ./...
VERSION ?= dev

.PHONY: build test lint fmt vet tidy clean

## build: compile all packages and produce the supervisor binary
build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY) ./cmd/$(BINARY)

## test: run the Go test suite
test:
	go test $(PKG)

## lint: static analysis via golangci-lint
lint:
	golangci-lint run

## fmt: format all Go sources in place
fmt:
	gofmt -w .

## vet: lightweight static checks
vet:
	go vet $(PKG)

## tidy: prune and re-pin module dependencies
tidy:
	go mod tidy

## clean: remove build artifacts
clean:
	rm -rf bin
