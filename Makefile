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

.PHONY: build build-web test test-scripts test-web lint lint-web fmt fmt-web vet tidy clean generate proto-lint

## generate: run code generation tools (sqlc, buf)
generate:
	sqlc generate
	buf generate proto

## proto-lint: lint protobuf schemas via buf
proto-lint:
	buf lint proto

## build-web: compile Vite/React frontend SPA into web/dist
build-web:
	cd web && pnpm run build

## build: compile all packages and produce the supervisor binary
build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY) ./cmd/$(BINARY)

## test: run the Go test suite
test:
	go test $(PKG)

## test-web: run frontend Vitest suite
test-web:
	cd web && pnpm test

## lint-web: run frontend Oxlint and Oxfmt checks
lint-web:
	cd web && pnpm run lint && pnpm run format:check

## fmt-web: format frontend sources in place with oxfmt
fmt-web:
	cd web && pnpm run format

## test-scripts: run unit tests for runner image scripts
test-scripts:
	bash tests/unit/entrypoint_test.sh

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
