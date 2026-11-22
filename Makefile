.PHONY: all build run test fmt vet check install uninstall clean check-conflicts snapshot release help

BINARY_NAME = asciidraw
GOBIN := $(shell go env GOPATH)/bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

# GitHub token for `make release`: publishes the GitHub release and pushes the
# cask to wkoszek/homebrew-tap.
GITHUB_TOKEN_REF = op://infra/github-pat-wkoszek/credential

all: check build

## build: compile ./asciidraw
build: check-conflicts
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) .

## run: build and start (make run FILE=mock.txt opens a file)
run: build
	./$(BINARY_NAME) $(FILE)

## test: run the tests
test:
	go test ./...

## fmt: format the source
fmt:
	gofmt -w .

## vet: run go vet
vet:
	go vet ./...

## check: fail if unformatted, then vet and test
check: check-conflicts
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "✗ not gofmt'd (run make fmt):"; echo "$$unformatted" | sed 's/^/    /'; exit 1; \
	fi
	go vet ./...
	go test ./...

## install: install into $(GOBIN)
install: check-conflicts
	go install -ldflags "$(LDFLAGS)" .

## uninstall: remove the installed binary
uninstall:
	rm -f $(GOBIN)/$(BINARY_NAME)

## clean: remove build output
clean:
	go clean
	rm -f $(BINARY_NAME)
	rm -rf dist

## snapshot: build every package into dist/, unsigned; publish nothing
snapshot: check-conflicts
	HOMEBREW_TAP_TOKEN=unused MACOS_SIGN=0 goreleaser release --snapshot --clean

## release: publish the tag at HEAD to GitHub Releases and the Homebrew tap
release: check
	@git describe --tags --exact-match >/dev/null 2>&1 || { \
		echo "✗ HEAD has no tag. Tag it first: git tag -a v0.1.0 -m v0.1.0"; exit 1; }
	@token=$$(opc read -n $(GITHUB_TOKEN_REF)) && \
		GITHUB_TOKEN=$$token HOMEBREW_TAP_TOKEN=$$token goreleaser release --clean

# Syncthing leaves `<name>.sync-conflict-<date>-<id>.<ext>` next to a file two
# machines edited at once. A stray .go copy redeclares every symbol and the
# build dies with "X redeclared in this block", so name the files up front.
check-conflicts:
	@conflicts=$$(find . -path ./.git -prune -o -name '*.sync-conflict-*' -print); \
	if [ -n "$$conflicts" ]; then \
		echo "✗ Syncthing conflict copies present; resolve and delete them:"; \
		echo "$$conflicts" | sed 's/^/    /'; \
		exit 1; \
	fi

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
