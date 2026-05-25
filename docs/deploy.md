# Деплой

Этот файл про продовое развёртывание проекта.

Важно: для open-source репозитория **ничего не нужно отдельно "придумывать" для CI/CD**.  
В репе уже лежат GitHub Actions:

- `.github/workflows/ci.yml` — проверки, тесты, линтеры, сборка;
- `.github/workflows/deploy.yml` — автодеплой, если в репозитории настроены secrets.

То есть базовый сценарий такой:

- хочешь просто пользоваться проектом или форкнуть его — достаточно ручного деплоя;
- хочешь автодеплой по `push` в `main` — просто добавляешь secrets, workflow уже готов;
- если secrets не настроены, автодеплой сам по себе не нужен и ничего дополнительно редактировать не надо.

## Быстрый старт

### 1. Подготовить сервер

На чистом Ubuntu 22.04/24.04:

```bash
curl -fsSL https://raw.githubusercontent.com/Zhaba1337228/max-university-event-bot/main/scripts/setup-server.sh | bash
```

Скрипт:

- установит Docker и Docker Compose;
- настроит базовую защиту сервера;
- создаст рабочую директорию `/opt/app`;
- склонирует репозиторий;
- подготовит пользователя `deploy`;
- при желании сразу подготовит SSH-ключ под GitHub Actions.

### 2. Заполнить `.env.prod`

```bash
ssh deploy@<IP>
cd /opt/app
cp deployments/.env.prod.example .env.prod
nano .env.prod
```

Минимальный набор:

```env
DOMAIN=example.com
POSTGRES_PASSWORD=strong-password
MAX_BOT_TOKEN=token-from-max
ADMIN_SESSION_KEY=long-random-string
GIGACHAT_AUTH_KEY=...
GIGACHAT_CLIENT_ID=...
```

Если домена ещё нет, можно временно указать IP:

```env
DOMAIN=1.2.3.4
```

### 3. Первый деплой вручную

```bash
ssh deploy@<IP>
cd /opt/app
make deploy
```

`make deploy`:

- подтянет свежий git checkout;
- попробует скачать готовые образы;
- если образов нет или registry недоступен, соберёт локально;
- перезапустит продовые контейнеры.

Это основной и достаточный способ запуска. Для open-source проекта его хватает с головой.

## GitHub Actions

### Что уже есть в репе

Без дополнительных правок уже настроены:

- CI: тесты, race, coverage, `go vet`, `golangci-lint`, `next build`, миграции;
- deploy workflow: сборка образов, push в GHCR и SSH-деплой на сервер.

### Когда нужен автодеплой

Автодеплой имеет смысл, если:

- у тебя есть свой сервер;
- ты хочешь, чтобы `push` в `main` сам выкатывался в прод;
- ты готов хранить deploy secrets в GitHub repository settings.

Если проект просто open-source и нужен только CI, **ничего менять в workflow не надо**.  
Можно оставить всё как есть и просто не настраивать deploy secrets.

### Какие secrets нужны для автодеплоя

В GitHub:

`Settings -> Secrets and variables -> Actions`

Нужны:

| Secret | Значение |
| --- | --- |
| `DEPLOY_HOST` | IP или домен сервера |
| `DEPLOY_USER` | обычно `deploy` |
| `DEPLOY_PORT` | обычно `22` |
| `DEPLOY_KEY` | приватный SSH-ключ для входа на сервер |

После этого `deploy.yml` начнёт работать без дополнительных правок.

### Что делает deploy workflow

Цепочка такая:

```text
push -> CI -> build/push images -> SSH deploy -> healthcheck
```

Конкретно:

1. прогоняется `.github/workflows/ci.yml`;
2. если всё зелёное, собираются образы `app` и `web`;
3. образы публикуются в GHCR;
4. по SSH запускается `scripts/deploy.sh --no-build`;
5. после выката проверяется `/healthz`.

## Откат

Если нужен rollback:

```bash
ssh deploy@<IP>
cd /opt/app
git log --oneline -5
git reset --hard <commit>
make deploy-no-build
```

## HTTPS

Когда появляется домен:

1. укажи `A`-запись на IP сервера;
2. обнови `.env.prod`:

```env
DOMAIN=bot.example.com
MAX_BOT_WEBHOOK_URL=https://bot.example.com/webhook/max
ADMIN_WEB_BASE_URL=https://bot.example.com
```

3. заново выполни:

```bash
make deploy
```

Дальше `deploy.sh` сам подготовит конфиг Caddy и включит HTTPS.

## Что открыто наружу

Снаружи нужны только:

- `22` — SSH;
- `80` — HTTP;
- `443` — HTTPS.

Не должны быть открыты наружу:

- `5432` — PostgreSQL;
- `8081` — admin API;
- внутренние docker-сервисы.

## Шпаргалка

```bash
make deploy
make deploy-no-build
make deploy-logs
make prod-down
make prod-logs
```
