#!/usr/bin/env bash
# =============================================================================
# deploy.sh — быстрый деплой MAX University Event Bot на сервер
#
# Использование:
#   ./scripts/deploy.sh             # pull + rebuild + restart
#   ./scripts/deploy.sh --no-build  # только restart (если образы уже собраны)
#   ./scripts/deploy.sh --logs      # показать логи после запуска
#
# Требования на сервере:
#   - Docker >= 24, Docker Compose plugin
#   - .env.prod в корне репозитория (скопировать из deployments/.env.prod.example)
#   - Порт 80 открыт в firewall (для IP-деплоя)
#   - Порты 80 + 443 для HTTPS-деплоя с доменом
# =============================================================================

set -euo pipefail

COMPOSE="docker compose -f deployments/docker-compose.prod.yml"
ENV_FILE=".env.prod"
NO_BUILD=false
SHOW_LOGS=false

# ── Аргументы ────────────────────────────────────────────────────────────────
for arg in "$@"; do
  case "$arg" in
    --no-build) NO_BUILD=true ;;
    --logs)     SHOW_LOGS=true ;;
    --help|-h)
      echo "Использование: $0 [--no-build] [--logs]"
      exit 0 ;;
  esac
done

# ── Цвета ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${CYAN}[deploy]${NC} $*"; }
ok()    { echo -e "${GREEN}[  ok  ]${NC} $*"; }
warn()  { echo -e "${YELLOW}[ warn ]${NC} $*"; }
error() { echo -e "${RED}[error ]${NC} $*" >&2; }

# ── Проверка .env.prod ────────────────────────────────────────────────────────
if [ ! -f "$ENV_FILE" ]; then
  error ".env.prod не найден."
  echo "  Создай его: cp deployments/.env.prod.example .env.prod"
  echo "  Минимум: задай DOMAIN, MAX_BOT_TOKEN, POSTGRES_PASSWORD, ADMIN_SESSION_KEY"
  exit 1
fi

# Предупреждение если остались заглушки CHANGE_ME
if grep -q "CHANGE_ME" "$ENV_FILE"; then
  error "В .env.prod остались незаполненные CHANGE_ME:"
  grep -n "CHANGE_ME" "$ENV_FILE" | head -10
  exit 1
fi

# ── Читаем нужные переменные из .env.prod ────────────────────────────────────
# shellcheck disable=SC2046
eval "$(grep -v '^#' "$ENV_FILE" | grep -v '^$' | grep -E '^(DOMAIN|MAX_BOT_MODE)=' | sed 's/^/export /'  2>/dev/null || true)"

# ── Определяем режим (IP vs домен) ───────────────────────────────────────────
is_ip() {
  echo "$1" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+(:[0-9]+)?$'
}

DOMAIN="${DOMAIN:-}"
MODE="${MAX_BOT_MODE:-longpoll}"

echo ""
info "Деплой MAX University Event Bot"
echo "  Адрес:   ${DOMAIN:-<не задан>}"
echo "  Режим:   ${MODE}"

if is_ip "${DOMAIN:-}"; then
  echo "  TLS:     HTTP (без сертификата, IP-адрес)"
  echo ""
  warn "Используется IP-адрес. Бот будет работать в режиме ${MODE}."
  if [ "$MODE" = "webhook" ]; then
    warn "Webhook требует HTTPS. Для IP-деплоя лучше поставить MAX_BOT_MODE=longpoll в .env.prod"
  fi
elif [ -n "${DOMAIN:-}" ]; then
  # Проверяем, включён ли http:// в Caddyfile (HTTP-only)
  if grep -q '^http://' deployments/Caddyfile 2>/dev/null; then
    echo "  TLS:     HTTP (домен без сертификата)"
  else
    echo "  TLS:     HTTPS (Let's Encrypt)"
  fi
fi
echo ""

# ── Git pull ──────────────────────────────────────────────────────────────────
info "Обновляем код из git..."
git pull --ff-only
ok "Код обновлён: $(git log -1 --pretty='%h %s')"

# ── Build ─────────────────────────────────────────────────────────────────────
if [ "$NO_BUILD" = false ]; then
  info "Собираем Docker-образы (1-3 мин при первом запуске)..."
  # Пробрасываем DOMAIN в compose для Caddyfile
  DOMAIN="$DOMAIN" $COMPOSE build --pull
  ok "Образы собраны."
fi

# ── Запуск ────────────────────────────────────────────────────────────────────
info "Запускаем сервисы..."
DOMAIN="$DOMAIN" $COMPOSE up -d --remove-orphans
ok "Сервисы запущены."

# ── Healthcheck ───────────────────────────────────────────────────────────────
info "Ожидаем готовности (до 60 сек)..."
for i in $(seq 1 12); do
  sleep 5
  BOT_ID=$(DOMAIN="$DOMAIN" $COMPOSE ps -q bot 2>/dev/null || true)
  if [ -n "$BOT_ID" ]; then
    BOT_HEALTH=$(docker inspect --format='{{.State.Health.Status}}' "$BOT_ID" 2>/dev/null || echo "unknown")
    if [ "$BOT_HEALTH" = "healthy" ]; then
      ok "Бот готов."
      break
    fi
  fi
  if [ "$i" -eq 12 ]; then
    warn "Timeout healthcheck. Логи: make prod-logs"
  fi
done

# ── Статус ────────────────────────────────────────────────────────────────────
echo ""
info "Статус контейнеров:"
DOMAIN="$DOMAIN" $COMPOSE ps

echo ""
ok "Деплой завершён!"
if [ -n "${DOMAIN:-}" ]; then
  if grep -q '^http://' deployments/Caddyfile 2>/dev/null || is_ip "${DOMAIN:-}"; then
    echo -e "  Админка: ${CYAN}http://${DOMAIN}${NC}"
  else
    echo -e "  Админка: ${CYAN}https://${DOMAIN}${NC}"
  fi
fi

# ── Логи (опционально) ────────────────────────────────────────────────────────
if [ "$SHOW_LOGS" = true ]; then
  echo ""
  info "Логи (Ctrl+C для выхода):"
  DOMAIN="$DOMAIN" $COMPOSE logs -f --tail=50
fi
