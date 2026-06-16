run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

docker-up:
	docker compose up -d

docker-down:
	docker compose down

migrate:
	docker compose exec postgres psql -U blog -d blog -f /dev/stdin < migrations/001_create_posts.sql

tidy:
	go mod tidy
