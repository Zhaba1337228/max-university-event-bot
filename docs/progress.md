# Progress tracker — MAX University Event Bot

> Чеклист по `execution_plan.md` §23 (Дорожная карта на 20 дней).
> Обновляется по мере прохождения дней. Используется как «галочки в плане»
> в дополнение к `docs/deviations.md`.

Легенда: ✅ done · ⏳ in progress · ⬜ pending · 🚫 skipped (с обоснованием)

---

## День 1 — продуктовая рамка и репозиторий ✅

- [x] репозиторий `max-university-event-bot` (GitHub, private)
- [x] `go mod init github.com/Zhaba1337228/max-university-event-bot`
- [x] скелет директорий (cmd/, internal/, migrations/, deployments/, scripts/, docs/)
- [x] README.md с описанием стека и команд
- [x] `.env.example`, `.gitignore`, `.dockerignore`, `.editorconfig`, `.gitattributes` (LF), `.golangci.yml`, `Makefile`
- [x] cmd/bot/main.go и cmd/migrate/main.go — заглушки, собираются
- [x] docs/deviations.md — журнал расхождений
- [x] commit + push origin/main

**Артефакт:** `go build ./...` зелёный, оба бинаря печатают "Day 1 stub". ✅

---

## День 2 — UX и тексты ✅

- [x] подтянут MAX SDK v1.6.17, проверен API `Keyboard`/`AddCallback`
- [x] domain типы: User, Event/EventStats, Registration, ActionLog, Notification (+ методы)
- [x] internal/bot/fsm/states.go — все состояния + IsTextInput
- [x] internal/bot/fsm/context.go — UserFSMContext с omitempty JSON
- [x] internal/bot/callbacks/payloads.go — формат "group:action:args" + конструкторы
- [x] internal/bot/callbacks/payloads_test.go — round-trip тест на 36 payload'ов + edge cases
- [x] internal/bot/messages/ru.go — все шаблоны на русском без эмодзи
- [x] internal/bot/messages/format.go — Format*, HumanStatus/Format/EventStatus в Europe/Moscow
- [x] internal/bot/keyboards/{main_menu,events,registration,organizer,admin,common}.go
- [x] internal/bot/keyboards/keyboards_test.go — smoke-тесты структуры
- [x] internal/bot/messages/format_test.go — таблицы статусов, no-emoji guard
- [x] domain_test.go — User.HasConsent, IsOrganizer/IsAdmin, Status.IsActive/IsCancelled, Event.IsOpenForRegistration
- [x] commit + push

**Артефакт:** `go test ./... -race` зелёный; coverage: callbacks 100%, domain 100%, fsm 92.3%, keyboards 56.3%, messages 40.9%. ✅

---

## День 3 — БД и репозитории ✅

- [x] подтянуты pgx/v5, pgxpool, pgx/stdlib, goose/v3, godotenv, env/v11, pgxmock/v4, google/uuid
- [x] миграции (10 файлов, migrations/*.sql):
  - [x] 01_init_users — CHECK на role
  - [x] 02_init_events — CHECK на format/status + idx
  - [x] 03_init_registrations — UNIQUE (user, event) + partial idx для waitlist
  - [x] 04_init_action_logs — FK ON DELETE SET NULL + 4 индекса
  - [x] 05_init_notifications — CHECK на type/status + idx по pending
  - [x] 06_init_user_states — FSM persistence
  - [x] 07_users_consent — 152-ФЗ
  - [x] 08_notifications_dedup — UNIQUE на (user, event, type, minute)
  - [x] 09_attendance_code — uuid + checkin_at/by + qr_sent_message_id
  - [x] 10_seed_demo_event — 3 демо-события
- [x] migrations/embed.go — встраивание через embed.FS
- [x] cmd/migrate/main.go — полноценный CLI (up/down/status/redo/version/reset/create)
- [x] internal/repo/postgres.go — NewPool + IsUniqueViolation + IsNoRows
- [x] internal/repo/tx.go — TxRunner.InTx (RepeatableRead)
- [x] internal/repo/interfaces.go — 6 контрактов
- [x] internal/repo/users.go — EnsureByMaxID, Get*, UpdateProfile, SetRole, GrantConsent, ForgetMe
- [x] internal/repo/events.go — Create, Get/ForUpdate, ListOpen, ListByOrganizer, ListUpcoming, Update*, Stats
- [x] internal/repo/registrations.go — Create (UPSERT), Get*, UpdateStatus, SetAttendanceCode, MarkAttended, Count*, NextWaitlist*
- [x] internal/repo/action_logs.go — Append, ListByUser/ByEvent
- [x] internal/repo/notifications.go — Schedule (silent dedup), PickDue, MarkSent/Failed/Skipped
- [x] internal/repo/user_states.go — Load/Save UPSERT/Reset/PurgeStaleBefore
- [x] pgxmock-тесты (30+ тестов в 5 файлах)
- [x] commit + push

**Артефакт:** `go test ./internal/repo/...` зелёный (44.2% coverage). Бинари собираются. CI обещает прогнать миграции на реальном Postgres. ✅

---

## День 4 — интеграция MAX Bot API (long polling), главное меню ⏳

- [x] internal/pkg/secret/ — Mask, MaskHeader, ConstantTimeEqual (+тесты, 100%)
- [x] internal/pkg/ptr/ — To/Deref/DerefOr (generics)
- [x] internal/pkg/logger/ — slog json/text + FromContext/WithContext
- [x] internal/pkg/retry/ — экспонента + jitter + IsRetryable (+6 тестов)
- [x] internal/app/config.go — все Config структуры + Validate + String() маскировка
- [x] internal/app/config_test.go — defaults, missing required, webhook/admin/rate validation, String() leak check
- [x] **.github/workflows/ci.yml** — 4 джобы: test, lint, security, migrate-against-real-postgres
- [x] internal/external/maxclient/client.go — обёртка над maxbot.Api с retry, SendText/Keyboard/AnswerCallback
- [x] internal/transport/longpoll/longpoll.go — GetUpdates loop → канал, back-pressure через select+ctx
- [x] internal/bot/fsm/manager.go — Load/Save/Reset поверх repo.UserStateRepo
- [x] internal/bot/dispatcher.go — recover(), семафор-пул, switch по типу update
- [x] internal/bot/handlers.go — RouteMessage (cmd → start/help, остальное → fallback) + RouteCallback (group → handler)
- [x] internal/bot/handlers/start.go — /start, /help, OnBotStarted, OnMainMenu (с AnswerCallback)
- [x] internal/bot/handlers/fallback.go — «не понял», closes spinner на устаревших кнопках
- [x] internal/app/app.go — DI цепочка, Run (longpoll), Shutdown (closes pgxpool)
- [x] cmd/bot/main.go — config + logger + signal.NotifyContext + graceful shutdown
- [ ] manual smoke: MAX_BOT_MODE=longpoll → /start в живом боте отвечает главным меню
- [ ] commit + push

**Артефакт:** `MAX_BOT_MODE=longpoll` + реальный токен → бот отвечает на /start. ⏳ (зависит от живого MAX_BOT_TOKEN; код готов)

---

## Дни 5-20 ⬜

- [ ] **День 5** — мероприятия: список + карточка
- [ ] **День 6** — запись: FSM consent → ФИО → контакт → направление → подтверждение
- [ ] **День 7** — повторная запись, capacity, waitlist
- [ ] **День 8** — отмена записи + waitlist promote
- [ ] **День 9** — история действий
- [ ] **День 10** — роль организатора и меню `/organizer`
- [ ] **День 11** — список участников + CSV-экспорт
- [ ] **День 12** — рассылка (без AI)
- [ ] **День 13** — backend admin REST API + JWT auth
- [ ] **День 14** — frontend Next.js: bootstrap, auth, dashboard, events, participants
- [ ] **День 15** — QR-коды в боте, страница check-in, AI rewrite в админке
- [ ] **День 16** — AI-сервисы в боте + напоминания
- [ ] **День 17** — обработка ошибок и устойчивость
- [ ] **День 18** — webhook-режим и полировка демо
- [ ] **День 19** — security hardening, SECURITY.md
- [ ] **День 20** — финальный прогон + резерв

---

## Чеклист готовности к демо (см. план §24) ⬜

Будет заполнен в день 20 после финального прогона. Сейчас — пусто.
