.PHONY: all web build test dev clean

all: build

web:
	cd web && pnpm install --frozen-lockfile && pnpm build

build: web
	go build -o bin/worldkeeper ./cmd/worldkeeper

test:
	go vet ./...
	go test ./...
	cd web && pnpm typecheck

# Go server without opening a browser; run `cd web && pnpm dev` alongside for hot reload.
dev:
	go run ./cmd/worldkeeper -no-browser

clean:
	rm -rf bin dist web/dist/assets web/dist/index.html
