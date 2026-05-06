DATABASE_URL ?= postgres://golangkanban:golangkanban@localhost:5432/golangkanban?sslmode=disable
ADDR ?= :8080
GOOSE := go run github.com/pressly/goose/v3/cmd/goose@v3.27.1
TEMPL := go run github.com/a-h/templ/cmd/templ@v0.3.1001

.PHONY: db-up db-down migrate-up migrate-down templ run storybook test

db-up:
	docker-compose up -d postgres

db-down:
	docker-compose stop postgres

migrate-up:
	$(GOOSE) -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	$(GOOSE) -dir migrations postgres "$(DATABASE_URL)" down

templ:
	$(TEMPL) generate

run: templ
	DATABASE_URL="$(DATABASE_URL)" ADDR="$(ADDR)" go run ./cmd/server

storybook: templ
	npm install --prefix storybook-server
	go run ./cmd/storybook

test:
	go test ./...
