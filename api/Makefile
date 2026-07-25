GOROOT ?= /opt/homebrew/Cellar/go/1.26.3/libexec
export GOROOT

DATABASE_URL ?= postgres://tryout:tryout@localhost:5433/tryout?sslmode=disable
MIGRATE = go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate

.PHONY: up down run build test vet lint migrate-up migrate-down migrate-create seed reset

up:
	docker compose -f ../docker-compose.yml up -d --wait

down:
	docker compose -f ../docker-compose.yml down

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./... -race

run:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/server

migrate-up:
	$(MIGRATE) -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	$(MIGRATE) -path migrations -database "$(DATABASE_URL)" down 1

# usage: make migrate-create NAME=add_orders
migrate-create:
	$(MIGRATE) create -ext sql -dir migrations -seq $(NAME)

seed:
	psql "$(DATABASE_URL)" -f seed/dev.sql

# Full local rebuild: containers, schema, fixtures.
reset: up migrate-up seed
