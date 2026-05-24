# Деплой и автодеплой

## Быстрый старт (с нуля до работающего сервера)

### 1. Настроить сервер (один раз)

На чистом Ubuntu 22.04/24.04 VPS от **root**:

```bash
curl -fsSL https://raw.githubusercontent.com/Zhaba1337228/max-university-event-bot/main/scripts/setup-server.sh | bash
```

Скрипт сам:
- установит Docker + Docker Compose
- создаст пользователя `deploy` с доступом к Docker
- настроит UFW (открыты только 22, 80, 443)
- включит fail2ban (защита SSH) и автоматические security-обновления
- склонирует репозиторий в `/opt/app`
- **сгенерирует и выведет SSH-ключ** для GitHub Actions

### 2. Добавить секреты в GitHub

В конце скрипта будет выведен приватный ключ. Добавь его в репозиторий:

**GitHub → Settings → Secrets and variables → Actions → New repository secret**

| Имя секрета | Значение |
|-------------|----------|
| `DEPLOY_HOST` | IP-адрес сервера, например `185.10.20.30` |
| `DEPLOY_USER` | `deploy` |
| `DEPLOY_PORT` | `22` |
| `DEPLOY_KEY` | Приватный ключ из вывода скрипта (всё от `-----BEGIN` до `-----END`) |

### 3. Заполнить .env.prod на сервере

```bash
ssh deploy@<IP>
nano /opt/app/.env.prod
```

Минимум:
```env
DOMAIN=185.10.20.30          # IP или домен — всё остальное автоопределяется
POSTGRES_PASSWORD=сильный_пароль
MAX_BOT_TOKEN=токен_из_MAX
ADMIN_SESSION_KEY=$(openssl rand -base64 32)
# MAX_BOT_MODE — не трогай, авто: IP→longpoll, домен→webhook
```

### 4. Первый деплой вручную

```bash
ssh deploy@<IP>
cd /opt/app && make deploy
```

---

## Автодеплой (GitHub Actions)

После шагов выше — **каждый push в `main` автоматически деплоится**:

```
push → CI (test + lint + build) → deploy (SSH → git pull → docker build → up)
```

Пайплайн: `.github/workflows/deploy.yml`

- CI должен пройти первым (`needs: ci`)
- Параллельные деплои блокируются (`concurrency`)
- После деплоя проверяется `/healthz`
- При падении выводится инструкция для ручного отката

### Ручной деплой из GitHub Actions

Вкладка **Actions → deploy → Run workflow** → опционально "Пропустить docker build".

### Откат

```bash
ssh deploy@<IP>
cd /opt/app
git log --oneline -5          # найти нужный коммит
git reset --hard abc1234       # откатиться
make deploy --no-build
```

---

## Архитектура сети (что открыто наружу)

```
Интернет
   │
   ├── :80  (HTTP)  ──→ Caddy ──→ web:3000 (Next.js)
   │                         └──→ bot:8080 (webhook)
   │
   ├── :443 (HTTPS) ──→ Caddy (только при домене, Let's Encrypt)
   │
   └── :22  (SSH)  ──→ только для деплоя

Закрыто снаружи:
   bot:8081  (Admin REST API)  — только через Next.js rewrite
   postgres:5432               — только внутри Docker-сети backend
```

---

## Переход на HTTPS (когда будет домен)

1. Установить `A`-запись: `домен → IP сервера`
2. В `.env.prod` обновить:
   ```env
   DOMAIN=bot.youruniversity.ru
   MAX_BOT_WEBHOOK_URL=https://bot.youruniversity.ru/webhook/max
   ADMIN_WEB_BASE_URL=https://bot.youruniversity.ru
   ```
3. `make deploy`

Всё остальное — автоматически:
- `deploy.sh` определяет домен → генерирует Caddy с HTTPS
- `MAX_BOT_MODE` переключается на `webhook`
- Порты 80+443 открываются
- Caddy получает сертификат через Let's Encrypt

Для возврата на IP — просто поставь `DOMAIN=1.2.3.4` и `make deploy`.

---

## Makefile (шпаргалка)

```bash
make deploy          # git pull + build + restart (prod)
make deploy-no-build # только restart (образы не пересобирать)
make deploy-logs     # deploy + показать логи
make prod-down       # остановить все контейнеры
make prod-logs       # хвост логов всех сервисов
```
