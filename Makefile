.PHONY: all web build test dev clean

all: build

web:
	cd web && npm ci && npm run build

build: web
	go build -o bin/worldkeeper ./cmd/worldkeeper

test:
	go vet ./...
	go test ./...
	cd web && npm run typecheck

# Go server without opening a browser; run `cd web && npm run dev` alongside for hot reload.
dev:
	go run ./cmd/worldkeeper -no-browser

clean:
	rm -rf bin dist web/dist/assets web/dist/index.html
