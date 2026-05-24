.PHONY: help build run dev test test-cover lint fmt vet tidy migrate-up migrate-down migrate-status docker-up docker-down deploy deploy-no-build deploy-logs prod-down prod-logs seed clean

GO ?= go
BIN_DIR := bin

help:
	@echo ""
	@echo "  ── Dev ────────────────────────────────────────────"
	@echo "  make build           собрать бинари bot и migrate"
	@echo "  make run             запустить bot локально"
	@echo "  make dev             docker compose up (dev: postgres + bot + web)"
	@echo "  make docker-up       docker compose up -d --build (dev)"
	@echo "  make docker-down     docker compose down (dev)"
	@echo ""
	@echo "  ── Production ─────────────────────────────────────"
	@echo "  make deploy          git pull + build + restart (prod)"
	@echo "  make deploy-no-build restart без пересборки образов"
	@echo "  make deploy-logs     deploy + хвост логов"
	@echo "  make prod-down       остановить prod"
	@echo "  make prod-logs       показать логи prod"
	@echo ""
	@echo "  ── Tests & Lint ───────────────────────────────────"
	@echo "  make test            go test ./... -race"
	@echo "  make test-cover      тесты + coverage отчёт"
	@echo "  make lint            golangci-lint run"
	@echo "  make fmt             gofmt + go mod tidy"
	@echo "  make vet             go vet"
	@echo ""
	@echo "  ── DB ─────────────────────────────────────────────"
	@echo "  make migrate-up      применить миграции"
	@echo "  make migrate-down    откатить последнюю"
	@echo "  make migrate-status  статус миграций"
	@echo "  make clean           удалить bin/ и coverage"
	@echo ""

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

# ── Production ───────────────────────────────────────────────────────────────
deploy:
	@bash scripts/deploy.sh

deploy-no-build:
	@bash scripts/deploy.sh --no-build

deploy-logs:
	@bash scripts/deploy.sh --logs

prod-down:
	docker compose --env-file .env.prod -f deployments/docker-compose.prod.yml down

prod-logs:
	docker compose --env-file .env.prod -f deployments/docker-compose.prod.yml logs -f --tail=100

seed:
	$(GO) run ./cmd/migrate up

clean:
	rm -rf $(BIN_DIR) coverage.out coverage.html
