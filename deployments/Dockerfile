# syntax=docker/dockerfile:1.7

# =============================================================================
# Multi-stage сборка:
#   builder — компилирует bot и migrate под Linux/amd64 без CGO.
#   runtime — alpine + ca-certificates + tzdata + non-root user.
#
# Особенности безопасности (см. SECURITY.md):
#   - non-root (USER app)
#   - ENTRYPOINT через /app/bot напрямую (без shell)
#   - tini не нужен — Go обрабатывает SIGTERM сам
# =============================================================================

FROM golang:1.25-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-mod=readonly

# Сначала только go.mod/go.sum для кэширования слоя зависимостей.
COPY go.mod go.sum ./
RUN go mod download

# Потом весь код.
COPY . .

# trimpath убирает абсолютные пути из бинаря (privacy).
# -s -w убирают symbol table и debug info (-30% размер).
RUN go build -trimpath -ldflags="-s -w" -o /out/bot     ./cmd/bot && \
    go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# =============================================================================
FROM alpine:3.20

# ca-certificates — для HTTPS к MAX и GigaChat.
# tzdata — чтобы FormatDateTime в Europe/Moscow работал.
RUN apk add --no-cache ca-certificates tzdata && \
    update-ca-certificates && \
    adduser -D -H -u 10001 app

WORKDIR /app
RUN mkdir -p /app/certs
COPY --from=builder /out/bot     /app/bot
COPY --from=builder /out/migrate /app/migrate
COPY migrations                  /app/migrations

ENV TZ=Europe/Moscow
USER app
EXPOSE 8080 8081

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/api/healthz || exit 1

ENTRYPOINT ["/app/bot"]
