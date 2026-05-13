.PHONY: help build run dev test test-cover lint fmt vet tidy migrate-up migrate-down migrate-status docker-up docker-down seed clean

GO ?= go
BIN_DIR := bin

help:
	@echo "make build         - собрать бинари bot и migrate"
	@echo "make run           - запустить bot локально"
	@echo "make dev           - docker compose up (postgres + bot)"
	@echo "make test          - go test ./... -race"
	@echo "make test-cover    - тесты + coverage отчёт"
	@echo "make lint          - golangci-lint run"
	@echo "make fmt           - gofmt + go mod tidy"
	@echo "make vet           - go vet"
	@echo "make migrate-up    - применить миграции"
	@echo "make migrate-down  - откатить последнюю"
	@echo "make migrate-status- статус миграций"
	@echo "make docker-up     - docker compose up -d --build"
	@echo "make docker-down   - docker compose down"
	@echo "make clean         - удалить bin/ и coverage"

build:
	$(GO) build -trimpath -o $(BIN_DIR)/bot ./cmd/bot
	$(GO) build -trimpath -o $(BIN_DIR)/migrate ./cmd/migrate

run:
	$(GO) run ./cmd/bot

dev:
	docker compose -f deployments/docker-compose.yml up --build

test:
	$(GO) test ./... -race -count=1

test-cover:
	$(GO) test ./... -race -count=1 -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .
	$(GO) mod tidy

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

migrate-up:
	$(GO) run ./cmd/migrate up

migrate-down:
	$(GO) run ./cmd/migrate down

migrate-status:
	$(GO) run ./cmd/migrate status

docker-up:
	docker compose -f deployments/docker-compose.yml up -d --build

docker-down:
	docker compose -f deployments/docker-compose.yml down

seed:
	$(GO) run ./cmd/migrate up

clean:
	rm -rf $(BIN_DIR) coverage.out coverage.html
