.PHONY: web build build-mcp docker

ifeq ($(OS),Windows_NT)
  AIR_CONF = .air.windows.toml
else
  AIR_CONF = .air.linux.toml
endif

web:
	air -c $(AIR_CONF)

build:
	npm run build-css
	go run ./cmd/copyassets
	go build -o build/ ./cmd/web

build-mcp:
	go build -o build/ ./cmd/mcp

docker:
	docker build -f cmd/web/Dockerfile -t ct-go-chat .
	docker image prune -f