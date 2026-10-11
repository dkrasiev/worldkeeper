.PHONY: all web build test dev app install-app clean

all: build

# Local builds are versioned like dev builds, after the next minor release:
# 0.3.0-local.abc1234 after v0.2.0. That is older than the release itself,
# so once it is out the app offers it.
NEXT_MINOR := $(shell git describe --tags --abbrev=0 --match 'v*' 2>/dev/null | sed 's/^v//' | awk -F. '{ printf "%d.%d.0", $$1, $$2 + 1 }')
VERSION ?= $(or $(NEXT_MINOR),0.1.0)-local.$(shell git rev-parse --short HEAD)

web:
	cd web && pnpm install --frozen-lockfile && pnpm build

build: web
	go build -o bin/worldkeeper ./cmd/worldkeeper

test:
	go vet ./...
	go test ./...
	cd web && pnpm typecheck && pnpm test

# Go server without opening a browser; run `cd web && pnpm dev` alongside for hot reload.
dev:
	go run ./cmd/worldkeeper -no-browser

# macOS only: dist/Worldkeeper.app for this Mac's architecture (releases ship
# a universal one, built by GoReleaser; see tools/macapp).
app: web
	@[ "$$(uname)" = Darwin ] || { echo "make app needs macOS: the menu bar icon needs cgo"; exit 1; }
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o dist/worldkeeper ./cmd/worldkeeper
	go run ./tools/macapp -binary dist/worldkeeper -version $(VERSION) -app dist/Worldkeeper.app

# Replaces /Applications/Worldkeeper.app; quit Worldkeeper first. Settings
# and saves are not touched.
install-app: app
	rm -rf /Applications/Worldkeeper.app
	ditto dist/Worldkeeper.app /Applications/Worldkeeper.app

clean:
	rm -rf bin dist web/dist/assets web/dist/index.html cmd/worldkeeper/*.syso cmd/worldkeeper/winres/icon.ico
