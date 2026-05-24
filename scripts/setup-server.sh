#!/usr/bin/env bash
# =============================================================================
# setup-server.sh — одноразовая подготовка VPS для MAX University Event Bot
#
# Что делает:
#   1. Обновляет пакеты
#   2. Ставит Docker Engine + Compose plugin
#   3. Создаёт пользователя deploy (без root-доступа к системе, но с Docker)
#   4. Настраивает UFW: открывает 22/SSH, 80/HTTP, 443/HTTPS, всё остальное блокирует
#   5. Клонирует репозиторий в /opt/app
#   6. Создаёт заглушку .env.prod с инструкцией заполнить вручную
#
# Запуск (от root на чистом Ubuntu 22.04 / 24.04):
#   curl -fsSL https://raw.githubusercontent.com/Zhaba1337228/max-university-event-bot/main/scripts/setup-server.sh | bash
#
# Или после клонирования:
#   bash scripts/setup-server.sh
# =============================================================================

set -euo pipefail

REPO_URL="https://github.com/Zhaba1337228/max-university-event-bot.git"
APP_DIR="/opt/app"
DEPLOY_USER="deploy"

# ── Цвета ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${CYAN}[setup]${NC} $*"; }
ok()    { echo -e "${GREEN}[  ok  ]${NC} $*"; }
warn()  { echo -e "${YELLOW}[ warn ]${NC} $*"; }
error() { echo -e "${RED}[error ]${NC} $*" >&2; exit 1; }

# ── Только root ───────────────────────────────────────────────────────────────
if [ "$(id -u)" -ne 0 ]; then
  error "Запусти от root: sudo bash $0"
fi

echo ""
info "=== Настройка сервера MAX University Event Bot ==="
echo ""

# ── 1. Обновление пакетов ─────────────────────────────────────────────────────
info "Обновляем пакеты..."
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get upgrade -y -qq
apt-get install -y -qq \
  ca-certificates curl gnupg git ufw fail2ban \
  unattended-upgrades apt-listchanges
ok "Пакеты обновлены."

# ── 2. Docker Engine ──────────────────────────────────────────────────────────
info "Устанавливаем Docker..."
if command -v docker &>/dev/null; then
  ok "Docker уже установлен: $(docker --version)"
else
  # Официальный способ по docs.docker.com/engine/install/ubuntu/
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg

  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
     https://download.docker.com/linux/ubuntu \
     $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
    > /etc/apt/sources.list.d/docker.list

  apt-get update -qq
  apt-get install -y -qq \
    docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

  systemctl enable --now docker
  ok "Docker $(docker --version) установлен."
fi

# ── 3. Пользователь deploy ────────────────────────────────────────────────────
info "Создаём пользователя '${DEPLOY_USER}'..."
if id "${DEPLOY_USER}" &>/dev/null; then
  ok "Пользователь '${DEPLOY_USER}' уже существует."
else
  useradd -m -s /bin/bash "${DEPLOY_USER}"
  ok "Пользователь '${DEPLOY_USER}' создан."
fi

# Добавить в группу docker (без sudo для запуска контейнеров)
usermod -aG docker "${DEPLOY_USER}"
ok "'${DEPLOY_USER}' добавлен в группу docker."

# ── 4. SSH-директория для deploy (для GitHub Actions) ────────────────────────
info "Настраиваем SSH для '${DEPLOY_USER}'..."
SSH_DIR="/home/${DEPLOY_USER}/.ssh"
mkdir -p "$SSH_DIR"
chmod 700 "$SSH_DIR"
touch "${SSH_DIR}/authorized_keys"
chmod 600 "${SSH_DIR}/authorized_keys"
chown -R "${DEPLOY_USER}:${DEPLOY_USER}" "$SSH_DIR"

# Генерируем пару ключей для GitHub Actions (если ещё не создана)
DEPLOY_KEY="${SSH_DIR}/github_actions"
if [ ! -f "${DEPLOY_KEY}" ]; then
  ssh-keygen -t ed25519 -C "github-actions-deploy" -f "${DEPLOY_KEY}" -N ""
  cat "${DEPLOY_KEY}.pub" >> "${SSH_DIR}/authorized_keys"
  ok "SSH-ключ для GitHub Actions сгенерирован."
  echo ""
  warn "━━━ СКОПИРУЙ ПРИВАТНЫЙ КЛЮЧ В GITHUB SECRETS ━━━"
  echo "  Имя секрета: DEPLOY_KEY"
  echo "  Значение (весь вывод ниже):"
  echo ""
  cat "${DEPLOY_KEY}"
  echo ""
  warn "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
else
  ok "SSH-ключ уже существует."
fi

# ── 5. UFW — файервол ─────────────────────────────────────────────────────────
info "Настраиваем UFW..."
ufw --force reset        > /dev/null
ufw default deny incoming
ufw default allow outgoing

ufw allow 22/tcp comment  "SSH"
ufw allow 80/tcp comment  "HTTP"
ufw allow 443/tcp comment "HTTPS"

ufw --force enable
ok "UFW настроен:"
ufw status numbered

# ── 6. fail2ban — защита SSH от брутфорса ────────────────────────────────────
info "Настраиваем fail2ban..."
systemctl enable --now fail2ban
# Минимальная конфигурация для SSH
cat > /etc/fail2ban/jail.d/ssh-custom.conf << 'EOF'
[sshd]
enabled  = true
port     = ssh
maxretry = 5
bantime  = 3600
findtime = 600
EOF
systemctl reload fail2ban
ok "fail2ban активен."

# ── 7. Автоматические security-обновления ────────────────────────────────────
info "Включаем автообновления безопасности..."
cat > /etc/apt/apt.conf.d/50unattended-upgrades-custom << 'EOF'
Unattended-Upgrade::Allowed-Origins {
    "${distro_id}:${distro_codename}-security";
};
Unattended-Upgrade::AutoFixInterruptedDpkg "true";
Unattended-Upgrade::MinimalSteps "true";
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Automatic-Reboot "false";
EOF
systemctl enable --now unattended-upgrades
ok "Автообновления включены."

# ── 8. Клонирование репозитория ───────────────────────────────────────────────
info "Клонируем репозиторий в ${APP_DIR}..."
if [ -d "${APP_DIR}/.git" ]; then
  ok "Репозиторий уже клонирован. Выполняем git pull..."
  cd "${APP_DIR}"
  git pull --ff-only
else
  git clone "${REPO_URL}" "${APP_DIR}"
  ok "Репозиторий склонирован."
fi
chown -R "${DEPLOY_USER}:${DEPLOY_USER}" "${APP_DIR}"

# ── 9. Шаблон .env.prod ───────────────────────────────────────────────────────
if [ ! -f "${APP_DIR}/.env.prod" ]; then
  cp "${APP_DIR}/deployments/.env.prod.example" "${APP_DIR}/.env.prod"
  chown "${DEPLOY_USER}:${DEPLOY_USER}" "${APP_DIR}/.env.prod"
  ok "Создан .env.prod из шаблона — заполни его!"
else
  ok ".env.prod уже существует."
fi

# ── Итог ─────────────────────────────────────────────────────────────────────
echo ""
ok "━━━ Сервер настроен! ━━━"
echo ""
echo "  Следующие шаги:"
echo ""
echo "  1. Добавь в GitHub Secrets (Settings → Secrets → Actions):"
echo "     DEPLOY_HOST   = $(curl -s ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')"
echo "     DEPLOY_USER   = ${DEPLOY_USER}"
echo "     DEPLOY_PORT   = 22"
echo "     DEPLOY_KEY    = (см. вывод выше — приватный ключ)"
echo ""
echo "  2. Заполни .env.prod на сервере:"
echo "     nano ${APP_DIR}/.env.prod"
echo ""
echo "  3. Первый запуск:"
echo "     su - ${DEPLOY_USER} -c 'cd ${APP_DIR} && make deploy'"
echo ""
echo "  После этого любой push в main автоматически задеплоится через GitHub Actions."
echo ""
