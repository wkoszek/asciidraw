.PHONY: all build run test fmt vet check install uninstall clean check-conflicts snapshot rel release notes next changelog hooks help

BINARY_NAME = asciidraw
GOBIN := $(shell go env GOPATH)/bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

# GitHub token for `make release`: publishes the GitHub release and pushes the
# cask to wkoszek/homebrew-tap.
GITHUB_TOKEN_REF = op://infra/github-pat-wkoszek/token
# Gitea token: asks tig to push the new tag to the GitHub mirror right away.
GITEA_TOKEN_REF = op://infra/gitea-pat-wkoszek/token

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
	rm -rf dist build

## snapshot: build every package into dist/, unsigned; publish nothing
snapshot: check-conflicts
	HOMEBREW_TAP_TOKEN=unused MACOS_SIGN=0 goreleaser release --snapshot --clean

## hooks: check commit messages in this clone (run once)
hooks:
	git config core.hooksPath .githooks

## next: show the next version and the commits that decide it
next:
	@scripts/release.sh log

## changelog: draft the next version's CHANGELOG.md section from the commits
changelog:
	@scripts/release.sh draft

## rel: the whole release: draft the changelog, edit it, commit, push, publish
rel:
	@[ -z "$$(git status --porcelain)" ] || { echo "✗ Commit or stash your changes first."; exit 1; }
	@if ! git describe --tags --exact-match --match 'v[0-9]*' HEAD >/dev/null 2>&1; then \
		v=$$(scripts/release.sh next) || exit 1; \
		if ! grep -q "^## $$v " CHANGELOG.md; then \
			scripts/release.sh draft && $${EDITOR:-vi} CHANGELOG.md && \
			git commit -q -m "docs: changelog for $$v" CHANGELOG.md || exit 1; \
		fi; \
	fi
	git push -q tig main
	@$(MAKE) --no-print-directory release

## release: tag the next version, push the tag, publish everywhere
release: check
	@GITEA_TOKEN_REF=$(GITEA_TOKEN_REF) GITHUB_TOKEN_REF=$(GITHUB_TOKEN_REF) \
		scripts/release.sh tag
	@$(MAKE) --no-print-directory notes
	@token=$$(opc read -n $(GITHUB_TOKEN_REF)) && \
		GITHUB_TOKEN=$$token HOMEBREW_TAP_TOKEN=$$token \
		goreleaser release --clean --release-notes build/notes.md

## notes: write the CHANGELOG.md section for the tag at HEAD to build/notes.md
notes:
	@tag=$${TAG:-$$(git describe --tags --exact-match 2>/dev/null)}; \
	mkdir -p build; \
	awk -v head="## $$tag " 'index($$0, head) == 1 { on = 1; next } /^## / { if (on) exit } on' \
		CHANGELOG.md | sed '/./,$$!d' > build/notes.md; \
	[ -s build/notes.md ] || { echo "✗ CHANGELOG.md has no section for $$tag. Write one first."; exit 1; }; \
	echo "Release notes for $$tag: build/notes.md"

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
