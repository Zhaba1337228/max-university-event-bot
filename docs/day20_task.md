# День 20 — задача для cloud-агента

> Готовое ТЗ. Передать так:
> `devin.exe setup` (один раз, чтобы подключить GitHub) →
> в терминале Devin сказать: «Выполни задачу из docs/day20_task.md, ветка main».

## Контекст
- Repo: github.com/Zhaba1337228/max-university-event-bot (private)
- Стек: Go 1.25.7, PostgreSQL 16, Next.js 14 (web/), JWT HS256, gocron/v2,
  опц. GigaChat. Подробности — README.md, docs/deploy.md, docs/progress.md.
- Дни 1–19 + 21 + 14 + 16 завершены. CI зелёный (build/test/lint/security/migrate/web).

## Что сделать

### 1. Финальный smoke (чистое окружение)
- `go build ./...`, `go vet ./...`, `go test -race -count=1 ./...`
- `cd web && npm install --no-audit --no-fund && npm run build && npm run lint`
- `docker compose -f deployments/docker-compose.yml build` (postgres + migrate + bot + web)
- Убедиться, что migrate-up работает в контейнере (опционально: поднять с тестовой `MAX_BOT_TOKEN=fake` и проверить, что бот доходит до Ping и корректно умирает с понятной ошибкой).

Если что-то падает — починить минимально-инвазивно и закоммитить отдельным
коммитом. Существующий рабочий код не переписывать.

### 2. docs/demo_walkthrough.md (новый файл)
Пошаговый сценарий для жюри:
- Минимальный `.env` для запуска (MAX_BOT_TOKEN, ADMIN_SESSION_KEY ≥32
  символов, DATABASE_URL; опц. GIGACHAT_AUTH_KEY).
- Сценарий абитуриента в боте: `/start` → меню → список → выбор события →
  consent → ФИО → контакт → направление → подтверждение → QR в чат.
- Сценарий организатора в боте: `/organizer` → событие → участники,
  CSV, рассылка, AI-сводка.
- Сценарий веб-админки: `/admin_login` в боте → клик по magic-ссылке →
  /auth → /dashboard → /events/:id → broadcast/close → /checkin (камера).
- Что показывается, если GigaChat выключен (fallback — это by design).
- Known issues и ограничения MVP.

### 3. docs/runbook.md (новый файл)
Шпаргалка эксплуатации:
- `make docker-up` / `make docker-down`, где смотреть логи (stdout JSON slog).
- Как добавить organizer'а: `ORGANIZER_USER_IDS=...` в .env + restart bot.
- 152-ФЗ: `/forget_me` в боте — что делать оператору на запросы пользователей.
- Если MAX webhook отписал бота (через 8 ч простоя) — логи
  `ensure subscription failed`, как восстановить.
- Ротация секретов: ADMIN_SESSION_KEY (инвалидирует все сессии),
  GIGACHAT_AUTH_KEY (без рестарта не подхватится — рестарт).

### 4. README.md
Добавить секцию «Демо» со ссылками на оба новых документа.

### 5. docs/progress.md
- Отметить День 20 как ✅.
- Заполнить «Чеклист готовности к демо» (план §24).

### 6. Коммит и push
```
docs(day20): final smoke + demo walkthrough + runbook
```
Дождаться зелёного CI (5 джобов).

### 7. Если останется время (опционально)
- Создать annotated tag `v0.1.0-demo` с release notes (что вошло в MVP).
- Заполнить `docs/deviations.md` любыми новыми расхождениями SDK/плана,
  если они всплывут на smoke.

## Жёсткие правила
- НЕ менять рабочий код без необходимости.
- НЕ пушить, пока CI не зелёный.
- НЕ публиковать секреты в commit message или коде.
- НЕ форсить push, не переписывать историю.
- Все коммиты — с трейлерами Devin (Generated with + Co-Authored-By).
