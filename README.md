# MAX University Event Bot

Чат-бот MAX для записи абитуриентов на мероприятия университета.
Кейс №2 хакатона. Стек: **Go + max-bot-api-client-go + PostgreSQL + GigaChat**,
веб-админка — **Next.js + shadcn/ui**.

> Связанные документы: [docs/demo_walkthrough.md](./docs/demo_walkthrough.md),
> [docs/runbook.md](./docs/runbook.md),
> [docs/deploy.md](./docs/deploy.md),
> [docs/progress.md](./docs/progress.md).

## Что умеет сейчас

**Бот**

- запись на мероприятие через диалог: согласие на ПДн -> ФИО -> направление интереса -> подтверждение;
- QR-код приглашения сразу после успешной записи;
- короткий код записи в карточке регистрации и в подписи к QR;
- повторная выдача QR через "Моя запись";
- статусы `registered` / `waitlist` / `cancelled` / `attended` / `no_show`;
- учёт мест и автопромоут из waitlist;
- напоминания о регистрации за 24 часа и за 1 час до начала;
- `/forget_me` с реальным каскадным удалением данных пользователя;
- AI-подбор мероприятий по интересу через GigaChat;
- AI FAQ: кнопка «Задать вопрос ИИ» — ответ на основе актуального расписания;
- персональная лента: после check-in бот присылает похожие мероприятия.

**Веб-админка**

- magic-link вход из бота через `/admin_login`;
- список своих мероприятий, создание и редактирование;
- AI-генерация описания события по названию и подсказке (кнопка «✨ Сгенерировать описание»);
- AI-подбор тегов по названию и описанию (кнопка «✨ Предложить теги»);
- список участников, поиск и фильтры;
- check-in по QR-коду или короткому коду записи;
- рассылка участникам;
- статистика по событию и AI-сводка.

## Напоминания о регистрации

Напоминания реализованы и живут в runtime, а не только в плане:

- расписание задаётся через `DEFAULT_EVENT_REMINDER_HOURS` в `.env`;
- scheduler из `internal/scheduler/scheduler.go`
  раз в несколько минут планирует `notifications` типов `reminder_24h` и `reminder_1h`;
- dispatch-job раз в минуту отправляет pending-уведомления в MAX;
- дедупликация идёт по `(user_id, event_id, type, minute_bucket)`, поэтому повторный прогон не создаёт дубли.

Если событие уже ближе, чем один из порогов, scheduler не создаёт просроченное напоминание.
Например, если до старта осталось меньше 24 часов, будет только `reminder_1h`.

## Стек

- **Backend:** Go 1.24+, PostgreSQL 16, `pgx/v5`, `goose`, `chi/v5`, `gocron/v2`, `slog`.
- **Frontend:** Next.js 14 (App Router), TypeScript, shadcn/ui, Tailwind, React Query.
- **AI:** GigaChat REST API.
- **Deploy:** Docker, docker compose, reverse proxy.

## Запуск в dev

```bash
git clone https://github.com/Zhaba1337228/max-university-event-bot.git
cd max-university-event-bot

cp .env.example .env
# заполнить MAX_BOT_TOKEN, ADMIN_SESSION_KEY и при необходимости GIGACHAT_AUTH_KEY

make docker-up
```

После старта:

- бот доступен в MAX по `/start`;
- веб-админка — на `http://localhost:3000`;
- admin API — на `http://localhost:8081`;
- healthcheck бота — `http://localhost:8080/healthz`.

## Полезные переменные окружения

```bash
AI_EVENT_RECOMMENDER_ENABLED=true
AI_NOTIFICATION_REWRITER_ENABLED=true
AI_ORGANIZER_SUMMARY_ENABLED=true
AI_FAQ_ENABLED=true   # AI FAQ в боте (кнопка «Задать вопрос ИИ»; false — кнопка скрыта)
AI_REQUEST_TIMEOUT=15s
AI_MAX_TOKENS=512
```

## Makefile

```bash
make help
make build
make run
make test
make lint
make migrate-up
make docker-up
make docker-down
```

## Структура проекта

- `cmd/bot/` — запуск бота и admin API;
- `cmd/migrate/` — оболочка над goose;
- `internal/app/` — сборка зависимостей и конфиг;
- `internal/bot/` — dispatcher, FSM, handlers, тексты, клавиатуры;
- `internal/domain/` — доменные сущности;
- `internal/repo/` — pgx-репозитории;
- `internal/service/` — бизнес-логика;
- `internal/scheduler/` — планирование и отправка напоминаний;
- `internal/transport/` — long-poll, webhook и admin REST API;
- `web/` — Next.js админка;
- `migrations/` — SQL-миграции;
- `deployments/` — Dockerfile и compose-файлы.

## Актуальный сценарий регистрации

1. Пользователь открывает список мероприятий.
2. Выбирает карточку события и нажимает "Записаться".
3. Даёт согласие на ПДн, если его ещё нет.
4. Вводит ФИО.
5. Указывает интересующее направление.
6. Подтверждает запись.
7. Получает текстовое подтверждение, короткий код и отдельным сообщением QR.
8. Позже получает напоминание за 24 часа и/или за 1 час до начала.

## Важные замечания

- Новые регистрации больше не собирают телефон и email.
- Старые поля контакта в БД и интерфейсах ещё могут встречаться как legacy-совместимость.
- Для событий без `ends_at` check-in открыт с `00:00` по Москве в день события и до `04:00` следующего дня.

## Документация

- [docs/demo_walkthrough.md](./docs/demo_walkthrough.md) — сценарий показа жюри;
- [docs/runbook.md](./docs/runbook.md) — эксплуатация и инциденты;
- [docs/deploy.md](./docs/deploy.md) — развёртывание;
- [docs/progress.md](./docs/progress.md) — прогресс по задачам.
