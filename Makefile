run-api:
	go run ./apps/api

run-cli:
	go run ./apps/cli

test:
	go test ./...

build:
	mkdir -p bin
	go build -o bin/ytvault-api ./apps/api
	go build -o bin/yd ./apps/cli

docker-build:
	docker build -t ytvault .

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f
