# gh-runner — AIO Supervisor
#
# All targets are intended to run inside the Nix development shell:
#   nix develop --command make <target>

# The supervisor is strictly CGO-free (docs/06 §1: pure-Go stack for
# seamless ARM64/AMD64 cross-compilation). Force it so host toolchains
# with a C compiler present (e.g. the Nix development shell) cannot sneak
# in dynamic glibc linking, which breaks test binaries whose interpreter
# path no longer exists in the Nix store.
export CGO_ENABLED := 0

BINARY := supervisor
PKG     := ./...

.PHONY: build test lint fmt vet tidy clean generate

## generate: run code generation tools (sqlc)
generate:
	sqlc generate

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
