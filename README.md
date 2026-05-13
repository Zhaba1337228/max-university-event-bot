# MAX University Event Bot

Чат-бот MAX для записи абитуриентов на мероприятия университета.
Кейс №2 хакатона. Стек: **Go + max-bot-api-client-go + PostgreSQL + GigaChat**, веб-админка — **Next.js + shadcn/ui**.

> Документация плана разработки: [`execution_plan.md`](./execution_plan.md), системный промт исполнителя — [`executor_prompt.md`](./executor_prompt.md), продуктовое описание кейса — [`max_case2_bot_plan.md`](./max_case2_bot_plan.md).

## Возможности (MVP)

**Бот:**

- запись на мероприятие через диалог: согласие на ПДн → ФИО → контакт → направление → подтверждение;
- QR-код приглашения присылается сразу после успешной записи;
- статусы `registered` / `waitlist` / `cancelled` / `attended` / `no_show`;
- учёт мест и лист ожидания с автопромоушеном;
- история действий пользователя;
- напоминания за 24 ч и 1 ч до старта;
- команда `/forget_me` — реальное удаление всех данных пользователя (152-ФЗ);
- AI-подбор мероприятия по интересу (GigaChat).

**Веб-админка (`:3000` Next.js, REST API на `:8081`):**

- magic-link логин из бота (`/admin_login` → ссылка → cookie-сессия);
- список своих мероприятий, создание/редактирование/закрытие;
- список участников с поиском и фильтрами;
- статистика и AI-сводка по событию;
- рассылка с предпросмотром и AI-улучшением текста;
- QR check-in: камера телефона → скан → `attended` за один скан.

## Стек

- **Backend:** Go 1.24+, `max-bot-api-client-go`, PostgreSQL 16, `pgx/v5`, `goose`, `chi/v5`, `golang-jwt/jwt/v5`, `gocron/v2`, `slog`.
- **Frontend:** Next.js 14 (App Router) + TypeScript + shadcn/ui + Tailwind + React Query.
- **AI:** GigaChat REST API.
- **Деплой:** Docker + docker-compose, Caddy/Nginx ingress.

## Запуск (dev)

```bash
# 1) клонировать
git clone https://github.com/Zhaba1337228/max-university-event-bot.git
cd max-university-event-bot

# 2) переменные окружения
cp .env.example .env
# заполнить MAX_BOT_TOKEN, ADMIN_SESSION_KEY, GIGACHAT_AUTH_KEY (опц.)

# 3) запуск через docker compose (postgres + bot + frontend)
make docker-up

# или локально без docker
make migrate-up && make run
```

После запуска бот доступен в MAX по `/start`, веб-админка — на `http://localhost:3000`.

## Команды Makefile

```bash
make help            # список команд
make build           # собрать бинари bot и migrate
make run             # запустить bot локально
make test            # go test ./... -race
make lint            # golangci-lint
make migrate-up      # применить миграции БД
make docker-up       # docker compose up -d --build
```

## Архитектура

- `cmd/bot/` — точка входа бота, поднимает transport + dispatcher + scheduler + admin API.
- `cmd/migrate/` — обёртка над goose для миграций PostgreSQL.
- `internal/transport/` — long-polling, webhook (MAX) и admin REST API (`chi`).
- `internal/bot/` — dispatcher, FSM, handlers, тексты, клавиатуры.
- `internal/domain/` — чистые доменные типы (без внешних зависимостей).
- `internal/repo/` — `pgx`-репозитории.
- `internal/service/` — бизнес-логика: регистрация, события, рассылка, attendance, QR, auth.
- `internal/external/` — клиенты MAX SDK и GigaChat.
- `internal/scheduler/` — `gocron` cron-задачи (напоминания, очистка).
- `frontend/` — Next.js admin UI.
- `migrations/` — SQL-миграции `goose`.
- `deployments/` — Dockerfile и docker-compose.

Подробная архитектура — [`execution_plan.md`](./execution_plan.md), раздел 4.

## Демо-сценарий

1. `/start` в боте → главное меню.
2. «Записаться» → выбрать «День открытых дверей ИТ» → согласие на ПДн → ФИО → email → направление → подтвердить.
3. Бот присылает QR-код картинкой.
4. От имени организатора: `/admin_login` → magic-link → веб-админка.
5. Дашборд → статистика мероприятия → AI-сводка.
6. Рассылка с AI-улучшением текста → подтверждение → доставка.
7. На странице check-in отсканировать QR с телефона участника → отметка `attended`.

Подробный пошаговый сценарий для жюри (минимальный `.env`, абитуриент / организатор / веб-админка, поведение при выключенном GigaChat, known issues) — [`docs/demo_walkthrough.md`](./docs/demo_walkthrough.md). Шпаргалка эксплуатации (логи, роли organizer'ов, ротация секретов `ADMIN_SESSION_KEY` / `GIGACHAT_AUTH_KEY`, восстановление подписки на webhook после 8-часового простоя MAX, реакция на запросы 152-ФЗ) — [`docs/runbook.md`](./docs/runbook.md).

## Безопасность

- Stateless JWT с httpOnly cookie, без серверных таблиц сессий.
- 152-ФЗ: согласие на ПДн перед первой записью, `/forget_me` каскадно удаляет данные.
- Constant-time проверка webhook secret, rate-limit per-user и global.
- PII маскируется в логах, организаторские эндпоинты защищены `RequireEventOwner`.
- Подробности — `SECURITY.md` (создаётся в день 19).

## Лицензия

Pre-release / hackathon MVP. Лицензия будет добавлена перед публикацией.
