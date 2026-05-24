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
#   - Порты 80 и 443 открыты в firewall
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

# ── Цвета для вывода ─────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${CYAN}[deploy]${NC} $*"; }
ok()    { echo -e "${GREEN}[  ok  ]${NC} $*"; }
warn()  { echo -e "${YELLOW}[ warn ]${NC} $*"; }
error() { echo -e "${RED}[error ]${NC} $*" >&2; }

# ── Проверка .env.prod ────────────────────────────────────────────────────────
if [ ! -f "$ENV_FILE" ]; then
  error ".env.prod не найден."
  echo "  Скопируй шаблон: cp deployments/.env.prod.example .env.prod"
  echo "  Заполни реальными значениями и запусти снова."
  exit 1
fi

# Предупреждение если остались заглушки CHANGE_ME
if grep -q "CHANGE_ME" "$ENV_FILE"; then
  error "В .env.prod остались незаполненные CHANGE_ME. Заполни их перед деплоем!"
  grep -n "CHANGE_ME" "$ENV_FILE" | head -10
  exit 1
fi

# ── Подгружаем переменные (только те, что нужны скрипту: DOMAIN) ─────────────
# shellcheck disable=SC2046
export $(grep -v '^#' "$ENV_FILE" | grep -v '^$' | grep 'DOMAIN=' | xargs 2>/dev/null || true)

info "Деплой MAX Bot на домен: ${DOMAIN:-<не задан>}"
echo ""

# ── Git pull ──────────────────────────────────────────────────────────────────
info "Обновляем код из git..."
git pull --ff-only
ok "Код обновлён."

# ── Build ─────────────────────────────────────────────────────────────────────
if [ "$NO_BUILD" = false ]; then
  info "Собираем Docker-образы (это займёт 1-3 минуты при первом запуске)..."
  $COMPOSE build --pull
  ok "Образы собраны."
fi

# ── Rotate: остановить старые контейнеры, поднять новые ──────────────────────
info "Перезапускаем сервисы..."
$COMPOSE up -d --remove-orphans
ok "Сервисы запущены."

# ── Healthcheck ───────────────────────────────────────────────────────────────
info "Ожидаем готовности сервисов (до 60 сек)..."
for i in $(seq 1 12); do
  sleep 5
  BOT_HEALTH=$(docker inspect --format='{{.State.Health.Status}}' \
    "$(docker compose -f deployments/docker-compose.prod.yml ps -q bot 2>/dev/null)" 2>/dev/null || echo "unknown")
  if [ "$BOT_HEALTH" = "healthy" ]; then
    ok "Бот готов (healthcheck passed)."
    break
  fi
  if [ "$i" -eq 12 ]; then
    warn "Healthcheck timeout. Проверь логи: docker compose -f deployments/docker-compose.prod.yml logs bot"
  fi
done

# ── Статус ────────────────────────────────────────────────────────────────────
echo ""
info "Статус контейнеров:"
$COMPOSE ps

echo ""
ok "Деплой завершён!"
if [ -n "${DOMAIN:-}" ]; then
  echo -e "  🌐 Админка: ${CYAN}https://${DOMAIN}${NC}"
fi

# ── Логи (опционально) ────────────────────────────────────────────────────────
if [ "$SHOW_LOGS" = true ]; then
  echo ""
  info "Логи (Ctrl+C для выхода):"
  $COMPOSE logs -f --tail=50
fi
