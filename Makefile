# Build, not run: a bare `make` must not start a server that never exits.
.DEFAULT_GOAL := build

run-api:
	go run ./apps/api

run-cli:
	go run ./apps/cli

test:
	go test ./...

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT)

build:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/ytvault-api ./apps/api
	go build -o bin/yd ./apps/cli

docker-build:
	docker build -t ytvault .

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f
