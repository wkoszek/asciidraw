.PHONY: all build run test fmt vet check install uninstall clean check-conflicts help

BINARY_NAME = asciidraw
GOBIN := $(shell go env GOPATH)/bin

all: check build

## build: compile ./asciidraw
build: check-conflicts
	go build -o $(BINARY_NAME) .

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
	go install .

## uninstall: remove the installed binary
uninstall:
	rm -f $(GOBIN)/$(BINARY_NAME)

## clean: remove build output
clean:
	go clean
	rm -f $(BINARY_NAME)

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
