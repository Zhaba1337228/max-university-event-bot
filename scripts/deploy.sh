#!/usr/bin/env bash
# =============================================================================
# deploy.sh — deploy MAX University Event Bot
#
# Usage:
#   ./scripts/deploy.sh             # git pull + image pull + restart, fallback to local build
#   ./scripts/deploy.sh --no-build  # git pull + image pull + restart, without local build fallback
#   ./scripts/deploy.sh --logs      # show logs after startup
#
# DOMAIN auto-detection:
#   IP address  → HTTP, longpoll, port 80 only
#   Domain name → HTTPS (Let's Encrypt), webhook, ports 80+443
#
# Just set DOMAIN= in .env.prod — everything else is automatic.
# =============================================================================

set -euo pipefail

COMPOSE="docker compose --env-file .env.prod -f deployments/docker-compose.prod.yml"
ENV_FILE=".env.prod"
NO_BUILD=false
SHOW_LOGS=false

# ── Args ──────────────────────────────────────────────────────────────────────
for arg in "$@"; do
  case "$arg" in
    --no-build) NO_BUILD=true ;;
    --logs)     SHOW_LOGS=true ;;
    --help|-h)
      echo "Usage: $0 [--no-build] [--logs]"
      exit 0 ;;
  esac
done

# ── Colors ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${CYAN}[deploy]${NC} $*"; }
ok()    { echo -e "${GREEN}[  ok  ]${NC} $*"; }
warn()  { echo -e "${YELLOW}[ warn ]${NC} $*"; }
error() { echo -e "${RED}[error ]${NC} $*" >&2; }

# ── Check .env.prod ───────────────────────────────────────────────────────────
if [ ! -f "$ENV_FILE" ]; then
  error ".env.prod not found."
  echo "  Create it: cp deployments/.env.prod.example .env.prod"
  echo "  Minimum: DOMAIN, MAX_BOT_TOKEN, POSTGRES_PASSWORD, ADMIN_SESSION_KEY"
  exit 1
fi

if grep -q "CHANGE_ME" "$ENV_FILE"; then
  error "Unfilled CHANGE_ME in .env.prod:"
  grep -n "CHANGE_ME" "$ENV_FILE" | head -10
  exit 1
fi

# ── Read DOMAIN from .env.prod ────────────────────────────────────────────────
DOMAIN="$(grep -v '^#' "$ENV_FILE" | grep -v '^$' | grep '^DOMAIN=' | head -1 | cut -d= -f2- | tr -d '"' | tr -d "'" || true)"
DOMAIN="${DOMAIN:-}"

# ── Auto-detect: IP vs Domain ─────────────────────────────────────────────────
is_ip() {
  echo "$1" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$'
}

is_ipv6() {
  echo "$1" | grep -qE '^\[?[0-9a-fA-F:]+\]?$'
}

if is_ip "${DOMAIN}" || is_ipv6 "${DOMAIN}" || [ -z "${DOMAIN}" ]; then
  USE_TLS=false
else
  USE_TLS=true
fi

# ── Auto-set MAX_BOT_MODE if not explicitly set ───────────────────────────────
CURRENT_MODE="$(grep -v '^#' "$ENV_FILE" | grep -v '^$' | grep '^MAX_BOT_MODE=' | head -1 | cut -d= -f2- | tr -d '"' | tr -d "'" || true)"
CURRENT_MODE="${CURRENT_MODE:-}"

if [ "$USE_TLS" = true ]; then
  # Domain + HTTPS → webhook is best
  if [ "$CURRENT_MODE" != "webhook" ] && [ "$CURRENT_MODE" != "longpoll" ]; then
    # Not explicitly set, auto-configure
    sed -i "s/^MAX_BOT_MODE=.*/MAX_BOT_MODE=webhook/" "$ENV_FILE"
    MODE="webhook"
    info "Auto-set MAX_BOT_MODE=webhook (domain + HTTPS detected)"
  else
    MODE="$CURRENT_MODE"
  fi
else
  # IP + HTTP → longpoll only
  if [ "$CURRENT_MODE" = "webhook" ]; then
    warn "webhook requires HTTPS. Forcing longpoll (IP detected)."
    sed -i "s/^MAX_BOT_MODE=.*/MAX_BOT_MODE=longpoll/" "$ENV_FILE"
    MODE="longpoll"
  else
    if [ "$CURRENT_MODE" != "longpoll" ]; then
      sed -i "s/^MAX_BOT_MODE=.*/MAX_BOT_MODE=longpoll/" "$ENV_FILE"
      info "Auto-set MAX_BOT_MODE=longpoll (IP detected)"
    fi
    MODE="longpoll"
  fi
fi

echo ""
info "Deploy MAX University Event Bot"
echo "  Address:  ${DOMAIN:-<not set>}"
echo "  Mode:     ${MODE}"
if [ "$USE_TLS" = true ]; then
  echo "  TLS:      HTTPS (Let's Encrypt)"
  echo "  Ports:    80, 443"
else
  echo "  TLS:      HTTP (no certificate)"
  echo "  Ports:    80"
fi
echo ""

# ── Generate Caddyfile ────────────────────────────────────────────────────────
CADDYFILE="deployments/Caddyfile.gen"

if [ "$USE_TLS" = true ]; then
  SITE_HEADER="${DOMAIN}"
  HSTS_LINE='Strict-Transport-Security "max-age=31536000; includeSubDomains"'
else
  SITE_HEADER="http://${DOMAIN}"
  HSTS_LINE=""
fi

cat > "$CADDYFILE" <<CADDYEOF
${SITE_HEADER} {

  # ── MAX webhook (only used when MAX_BOT_MODE=webhook) ──
  handle /webhook/* {
    reverse_proxy bot:8080 {
      header_up X-Real-IP {remote_host}
    }
  }

  # ── Healthcheck via admin API (works in both longpoll and webhook modes) ──
  handle /healthz {
    rewrite * /api/healthz
    reverse_proxy bot:8081
  }

  # ── Everything else → Next.js admin panel ──
  handle {
    reverse_proxy web:3000 {
      header_up X-Real-IP {remote_host}
    }
  }

  log {
    output stdout
    format json
    level  INFO
  }

  header {
    X-Frame-Options        "SAMEORIGIN"
    X-Content-Type-Options "nosniff"
    X-XSS-Protection       "1; mode=block"
    ${HSTS_LINE}
    -Server
  }
}
CADDYEOF

ok "Caddyfile generated (${USE_TLS:-false} → $([ "$USE_TLS" = true ] && echo 'HTTPS' || echo 'HTTP'))"

# ── Export TLS flag for docker-compose ─────────────────────────────────────────
export USE_TLS

# ── Git pull ──────────────────────────────────────────────────────────────────
info "Updating code from git..."
git pull --ff-only
ok "Code updated: $(git log -1 --pretty='%h %s')"

# ── Build ──────────────────────────────────────────────────────────────────────
if [ -z "${IMAGE_TAG:-}" ]; then
  export IMAGE_TAG="$(git rev-parse HEAD)"
fi
info "Using image tag: ${IMAGE_TAG}"

info "Pulling Docker images..."
if DOMAIN="$DOMAIN" $COMPOSE pull migrate bot web; then
  ok "Images pulled."
elif [ "$NO_BUILD" = true ]; then
  error "Failed to pull prebuilt images and --no-build is set."
  exit 1
else
  warn "Prebuilt images are unavailable for tag ${IMAGE_TAG}. Falling back to local build."
  DOMAIN="$DOMAIN" $COMPOSE build --pull
  ok "Images built locally."
fi

# ── Start ──────────────────────────────────────────────────────────────────────
info "Starting services..."
DOMAIN="$DOMAIN" $COMPOSE up -d --remove-orphans
ok "Services started."

# ── Healthcheck ────────────────────────────────────────────────────────────────
info "Waiting for readiness (up to 60s)..."
for i in $(seq 1 12); do
  sleep 5
  BOT_ID=$(DOMAIN="$DOMAIN" $COMPOSE ps -q bot 2>/dev/null || true)
  if [ -n "$BOT_ID" ]; then
    BOT_HEALTH=$(docker inspect --format='{{.State.Health.Status}}' "$BOT_ID" 2>/dev/null || echo "unknown")
    if [ "$BOT_HEALTH" = "healthy" ]; then
      ok "Bot is ready."
      break
    fi
  fi
  if [ "$i" -eq 12 ]; then
    warn "Timeout healthcheck. Logs: make prod-logs"
  fi
done

# ── Status ─────────────────────────────────────────────────────────────────────
echo ""
info "Container status:"
DOMAIN="$DOMAIN" $COMPOSE ps

echo ""
ok "Deploy complete!"
if [ -n "${DOMAIN:-}" ]; then
  if [ "$USE_TLS" = true ]; then
    echo -e "  Admin:  ${CYAN}https://${DOMAIN}${NC}"
  else
    echo -e "  Admin:  ${CYAN}http://${DOMAIN}${NC}"
  fi
fi

# ── Logs (optional) ───────────────────────────────────────────────────────────
if [ "$SHOW_LOGS" = true ]; then
  echo ""
  info "Logs (Ctrl+C to exit):"
  DOMAIN="$DOMAIN" $COMPOSE logs -f --tail=50
fi
