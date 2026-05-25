# Progress tracker — MAX University Event Bot

> Чеклист по первоначальной дорожной карте на 20 дней.
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

## День 5 — мероприятия (список + карточка) ✅

- [x] internal/service/errors.go — каталог доменных ошибок (Err*)
- [x] internal/service/event.go — Event service + EventWithFree (ListOpen, GetOpen, Get, Stats, ListByOrganizer)
- [x] internal/service/event_test.go — 6 behavior tests (NotFound, Closed, FreeSeats, no-negative, paging, missing stats)
- [x] internal/bot/handlers/events.go — OnCallback (list with offset/hasMore + show карточки)
- [x] FSM сохраняет Offset (back-кнопка) и CurrentEventID (передача в reg-handler Дня 6)
- [x] callbacks.GroupEvent роутится в bot/handlers.go RouteCallback
- [x] DI: eventSvc подключён в app.go, WaitlistEnabled пробрасывается в Handlers
- [x] commit + push

**Артефакт:** список открытых мероприятий с пагинацией + карточка со свободными местами и кнопками. ✅

---

## День 6 — запись: FSM consent → ФИО → контакт → направление → подтверждение ✅

- [x] service.User (EnsureProfile, GrantConsent с ActionLog, ForgetMe)
- [x] service.Registration.Register с транзакцией RepeatableRead (FOR UPDATE)
- [x] handlers/registration.go (полный FSM с consent шагом)
- [x] handlers/my_registration.go (/forget_me, двухшаговое подтверждение)
- [x] commit + push

---

## День 7 — повторная запись, capacity, waitlist ✅

- [x] service.Registration делает все проверки (см. День 6: ErrAlreadyRegistered, ErrNoSeats)
- [x] handlers/waitlist.go (wl:join делегирует RegistrationHandler.onStart, который через сервис кладёт в waitlist)
- [x] keyboards.EventCard показывает «Встать в лист ожидания» когда мест 0 + waitlistEnabled
- [x] RouteCallback: GroupWaitlist подключён

---

## День 8 — отмена записи + waitlist promote ✅

- [x] service.Registration.Cancel(regID, by) в транзакции FOR UPDATE
- [x] Auto-promote первого waitlist при освобождении места (с defensive повторной capacity-проверкой)
- [x] service.Registration.ListActiveByUser, GetActive
- [x] handlers/cancel.go (двухшаговая отмена с проверкой ownership через ListActiveByUser)
- [x] my_registration.onShow теперь грузит реальные регистрации + клавиатура с кнопкой «Отменить»

---

## День 9 — история действий ✅

- [x] service.ActionLog (ListByUser/ByEvent)
- [x] my_registration.onHistory грузит и рендерит последние 10 действий через messages.HistoryHeader/HistoryLine
- [x] подключение в DI

---

## День 10 — роль организатора и /organizer меню ✅

- [x] service.Role (Bootstrap по env, RequireOrganizer, RequireEventOwner)
- [x] Bootstrap вызывается на старте app.New (admin → admin, organizer → organizer; admin не понижается)
- [x] handlers/organizer.go (/organizer cmd + showMenu + showStats + AI-сводка заглушка)
- [x] RouteMessage: /organizer
- [x] RouteCallback: GroupOrg
- [x] keyboards.OrganizerEventActions (Участники/CSV/Рассылка/AI-сводка/Закрыть)
- [x] messages.OrganizerStats с детерминированной сортировкой top-interests

---

## День 11 — список участников + CSV-экспорт ✅

- [x] handlers/organizer_list.go:
  - orglist:show:<eventID>:<offset> — постраничный список (10/страница) с маскировкой контактов
  - orglist:csv:<eventID> — CSV как текст в чате (до 3500 символов, иначе обрезается с указанием)
- [x] keyboards.OrganizerParticipants (нав + Назад к мероприятию)
- [x] RouteCallback: GroupOrgList

---

## День 12 — рассылка (без AI) ✅

- [x] service.Notification.SendBroadcast (rate-limit между сообщениями, ActionLog notification_sent на каждого получателя, MarkSent/MarkFailed на дубль/ошибку)
- [x] handlers/organizer_notify.go (FSM organizer_notif_text → organizer_notif_confirm)
- [x] RouteMessage: text input в organizer_notif_text
- [x] RouteCallback: GroupOrgNotif (start/send/cancel/ai-заглушка)
- [x] keyboards.OrganizerNotifConfirm (Отправить/Отмена/Улучшить через AI)

---

## День 13 — backend admin REST API + JWT auth ✅

- [x] service/auth.go — Auth interface:
  - IssueMagic (5 мин, purpose=magic, доступ только organizer/admin → ErrAccessDenied)
  - IssueSession (12 ч, purpose=session, роль из БД)
  - VerifyMagic / VerifySession (сверка роли в БД на каждом запросе → ErrAuthRoleChanged)
  - HS256, ID=uuid.NewString(), ExpirationRequired в parser
- [x] internal/transport/adminapi/ — chi-роутер на :8081:
  - server.go: Deps + middleware chain (RequestID, recover, slogLogger
    БЕЗ query/body для PII, securityHeaders, CORS только из ADMIN_WEB_BASE_URL)
  - middleware.go: requireSession (cookie sid), originGuard для mutating,
    statusRecorder, writeJSON/errResp
  - handlers.go: /api/auth/exchange|logout|me, /api/events?status=open|mine,
    /api/events/:id (+ stats), /api/events/:id/participants (поиск/пагинация),
    /api/events/:id/open|close|broadcast, /api/checkin, /api/dashboard, /api/healthz
  - DTO с маскировкой full_name (Иванов И.) и contact
- [x] handlers/admin_login.go — /admin_login в боте:
  - IssueMagic → inline-кнопка-ссылка ${WebBaseURL}/auth?t=<jwt>
  - На applicant — AdminLoginNoAccess + main menu
  - Если ADMIN_SESSION_KEY не задан — handler пропускается через `if h.AdminLogin != nil`
- [x] app/app.go запускает adminAPI параллельно long-polling (только если authSvc != nil)

---

## День 14 — frontend Next.js ✅

- [x] web/ — Next.js 14 (App Router, standalone output), TypeScript, Tailwind
- [x] Без shadcn/ui CLI — лёгкие обёртки Card/Button/Input/Textarea/Badge на Tailwind
  (см. deviations.md, скорость сборки в Docker × отсутствие интерактивных шагов)
- [x] next.config.mjs rewrites /api/* → API_UPSTREAM (same-origin cookies)
- [x] /auth?t=<jwt> — Suspense + useSearchParams → POST /api/auth/exchange → /dashboard
- [x] /auth/login — fallback-страница «как войти»
- [x] (authenticated) route group с layout-guard на /api/auth/me
- [x] /dashboard — счётчики (events/registered/upcoming) + список своих
- [x] /events — табы «Мои/Все открытые»
- [x] /events/[id] — карточка + статистика + close/open + быстрые кнопки
- [x] /events/[id]/participants — пагинация + поиск + маскированные ПДн
- [x] /events/[id]/broadcast — textarea + лимит 4000 + confirm
- [x] /checkin — камера (@yudiel/react-qr-scanner) + anti-flood 3s + ручной ввод
- [x] web/Dockerfile (multi-stage node:20-alpine, non-root uid=10001, healthcheck)
- [x] deployments/docker-compose.yml — сервис web (read_only + tmpfs + cap_drop)
- [x] .github/workflows/ci.yml — новый job `web (next build)` на Node 20

**Артефакт:** `npm run build` зелёный (9/9 static pages), `npm run lint` без ошибок. ✅

---

## День 15 — QR-коды в боте + страница check-in ✅ (бот-часть)

- [x] service/qr.go — QR interface:
  - NewAttendanceCode (uuid v4 hex без дефисов, 32 символа, 128 бит энтропии)
  - BuildQRPayload (MAXUEB:<eventID>:<code>) + ParseQRPayload (с ErrQRInvalidPrefix/Format)
  - GenerateQRPNG (skip2/go-qrcode, Medium recovery, 512px)
- [x] service/attendance.go — Attendance interface:
  - CheckIn в транзакции RepeatableRead с GetByCodeForUpdate
  - Проверки: parsed.EventID == reg.EventID, RequireEventOwner, status active,
    окно [starts_at-2h, ends_at+4h]
  - MarkAttended + ActionLog checkin_scanned
  - На повторный скан → AlreadyDone=true (не ошибка)
- [x] handlers/registration.go::sendQRCode после успешной registered-записи:
  - NewAttendanceCode → SetAttendanceCode в БД
  - GenerateQRPNG → tmpfile → UploadPhotoFromFile → SendWithResult
  - Все ошибки логируются но не блокируют регистрацию
- [x] POST /api/checkin endpoint работает с реальными QR из бота
- [x] frontend страница /checkin реализована в Day 14 (камера + ручной ввод)

---

## День 17 — rate-limit per user ✅

- [x] internal/pkg/ratelimit/ — потокобезопасный per-key token bucket с TTL-эвикцией
- [x] Тесты: AllowConsumesTokens, AllowDifferentKeys, ConcurrentSafe, Size/Reset
- [x] Подключено в Dispatcher: rlText (2 rps, burst 5), rlCallback (5 rps, burst 10)
- [x] При срабатывании лимита — только warn-лог, без ответа пользователю (план §19.7)

---

## День 18 — webhook режим ✅

- [x] internal/transport/webhook/parser.go — ParseUpdate (дублирует приватный SDK
  bytesToProperUpdate для 5 типов: BotStarted, MessageCreated/Edited/Removed, Callback)
- [x] internal/transport/webhook/server.go — http.Server с:
  - POST /webhook/max + constant-time secret check (через secret.ConstantTimeEqual)
  - GET /healthz для k8s healthcheck
  - http.MaxBytesReader 1 MiB на body
  - ReadHeaderTimeout 5s, ReadTimeout/WriteTimeout/IdleTimeout
  - LRU-дедуп update_id (1024 элемента, TTL 10 мин)
  - На любую ошибку парсинга → 200 OK (иначе MAX отпишет через 8 ч простоя)
  - graceful shutdown 5s
- [x] app.go::ensureSubscription: при старте webhook-режима проверяет
  GetSubscriptions, если нашего URL нет — Subscribe с update_types
  [bot_started, message_created, message_callback]
- [x] app.Run переключает на webhook при MAX_BOT_MODE=webhook

---

## День 19 — security ✅

- [x] Документация по безопасности и ротации секретов оформлена,
  152-ФЗ flow, реакции на инцидент, известным trade-offs.
- [x] #nosec комментарии на 3 false-positive (G404 retry jitter, G101 docstring,
  G118 graceful shutdown context)
- [x] Все critical меры уже на месте: constant-time secret, маскировка PII,
  параметризованный SQL, RBAC, FSM-guard на двухшаговых подтверждениях,
  CSRF (Origin guard + SameSite=Strict), CORS, security headers.

---

## День 21 — Docker (бонус, по плану §21) ✅

- [x] deployments/Dockerfile — multi-stage golang:1.25-alpine → alpine:3.20,
  non-root USER app (uid=10001), trimpath+ldflags -s -w, healthcheck через /api/healthz
- [x] deployments/docker-compose.yml — postgres + migrate + bot;
  tmpfs:/tmp, cap_drop:ALL, no-new-privileges, env_file

---

## День 16 — AI (GigaChat) + scheduler ✅

- [x] internal/external/gigachat/client.go — OAuth `client_credentials` с
  автообновлением access_token (TTL ≥ 60s buffer), opt-in InsecureTLS для dev.
- [x] internal/external/gigachat/prompts.go — system/user шаблоны для
  Recommender / Rewriter / Summary.
- [x] internal/service/ai.go — фасад с graceful `ErrAIUnavailable`:
  - RecommendEvents — фильтрация выдуманных event_id, stripCodeFences.
  - RewriteNotification — лимит 4000 символов, JSON parsing fallback.
  - OrganizerSummary — детерминированная top_interests строка.
- [x] internal/scheduler/scheduler.go — gocron/v2, три job'а:
  - dispatchDue (1 мин) — PickDue + SendTextToUser + MarkSent/Failed,
    ~20 rps между отправками (50 ms sleep).
  - scheduleReminders (5 мин) — для событий в ближайшие 25h создаёт
    reminder_24h / reminder_1h на всех registered (uniq_notif_dedup
    делает операцию идемпотентной).
  - purgeStaleStates (24h) — чистка FSM старше StateTTL (7 дней).
- [x] handlers/ai_pick.go — главное меню → ai:pick → AIAskInterest →
  RecommendEvents → клавиатура «Записаться»; fallback в обычный список.
- [x] handlers/organizer.go::showAISummary — graceful fallback в обычную
  статистику + сообщение «AI недоступен».
- [x] handlers/organizer_notify.go::onAIRewrite — на fallback оставляет
  оригинальный текст.
- [x] app.go — GigaChat создаётся только при `GIGACHAT_AUTH_KEY != ""`,
  scheduler стартует всегда, корректный Stop() в Shutdown.
- [x] handlers.go — RouteCallback `GroupAI` + RouteMessage `StateAIPickIntent`.

**Артефакт:** build/vet/lint/test — все зелёные, без живого ключа GigaChat
вся AI-цепочка деградирует в fallback. ✅

---

## День 20 — финальный smoke + готовность к демо ✅

- [x] чистый clone → `go build ./...`, `go vet ./...`, `go test -race -count=1 ./...` зелёные
- [x] `cd web && npm install && npm run build && npm run lint` зелёные (Next.js 14.2.35, 0 warnings/errors)
- [x] `docker compose -f deployments/docker-compose.yml build` — postgres + migrate + bot + web собираются
- [x] smoke `docker compose up postgres migrate` — все 10 миграций применяются (включая seed демо-событий)
- [x] smoke бота с `MAX_BOT_TOKEN=fake` — корректно падает с `API error 401: verify.token (Invalid access_token)`, без panic, ровно как должно
- [x] `docs/demo_walkthrough.md` — продуктовый сценарий для жюри
- [x] `docs/runbook.md` — шпаргалка эксплуатации (роли, секреты, 152-ФЗ, webhook 8 ч)
- [x] README.md — секция «Демо» со ссылками на новые документы
- [ ] видео-демо как fallback (по плану §1 demo_walkthrough — записывается организатором презентации, не в репо)
- [x] финальная сверка `docs/deviations.md` — всё актуально, новых расхождений на smoke не выявлено
- [ ] RELEASE-tag `v0.1.0-demo` (опционально — после merge PR)

**Артефакт:** все 5 CI-джобов зелёные (test, lint, security, migrate, web). ✅

---

## Чеклист готовности к демо (см. план §24) ✅

### Технический (бот)

- [x] `make docker-up` поднимает всё с нуля (postgres + migrate + bot + web), миграции применяются автоматически (10 файлов)
- [ ] **токен MAX в `.env` — рабочий** — заполняется организатором стенда перед демо (см. `docs/demo_walkthrough.md` §0)
- [x] бот при старте делает `ping max api`, при невалидном токене падает с понятной ошибкой (smoke на fake-токене)
- [x] три seed-мероприятия видны в списке (`20260101000010_seed_demo_event.sql`)
- [x] первая запись требует согласие на обработку ПДн (`StateAwaitConsent`, фиксация `users.consent_policy_ver`)
- [x] полный сценарий запись → подтверждение → QR в чате → статус → отмена реализован (Days 6–8)
- [x] QR-картинка отправляется через `SendWithResult` + `qr_sent_message_id` (Day 15)
- [x] кнопка «Показать мой QR» в «Моих записях» перегенерирует картинку из `attendance_code`
- [x] `/forget_me` каскадно удаляет данные (`repo.Users.ForgetMe`, FK ON DELETE CASCADE/SET NULL)
- [x] AI-подбор возвращает ответ или корректно деградирует (`ErrAIUnavailable` → fallback в обычный список)
- [x] рассылка через scheduler-job `dispatchDue` (1 мин, RPS 20)
- [x] напоминания планируются `scheduleReminders` (5 мин tick, окно 25 ч, идемпотентно через `uniq_notif_dedup`)
- [x] graceful shutdown (контексты, `scheduler.Stop()` в `Shutdown`)

### Технический (веб-админка)

- [x] `/admin_login` в боте присылает magic-link (`service/auth.go`)
- [x] `/auth?t=<jwt>` → `POST /api/auth/exchange` → httpOnly cookie `session_jwt` → редирект на `/dashboard`
- [x] список своих событий + статистика на `/dashboard` (через `/api/events`)
- [x] страница `/events/[id]/participants` с поиском (debounce, серверная пагинация)
- [x] `/events/[id]/broadcast` — форма рассылки с «AI-улучшить»
- [x] `/checkin` — `@yudiel/react-qr-scanner` + ручной ввод attendance code
- [x] `POST /api/checkin` обрабатывает скан, ставит `attended`; повторный скан и чужое событие отвечают понятной ошибкой
- [x] `RequireEventOwner` middleware защищает event-scoped endpoints (organizer чужого события → 403)
- [x] CSRF Origin guard на mutating endpoints, SameSite=Strict на cookie
- [x] logout (`POST /api/auth/logout`) очищает cookie, повторный заход → `/auth/login`
- [x] ротация `ADMIN_SESSION_KEY` инвалидирует все живые сессии (документировано в `docs/runbook.md` §5)
- [x] `next build` без warnings TypeScript / ESLint (web-job CI зелёный)

### Безопасность и операции

- [x] логи структурированные (slog JSON), PII маскируется (`internal/pkg/secret.Mask*`)
- [x] webhook-secret валидируется `secret.ConstantTimeEqual`; `.env.example` явно требует 5..256 символов `[a-zA-Z0-9_-]`
- [x] `ADMIN_SESSION_KEY` ≥ 32 символа (валидация в `Config.Validate`)
- [x] `gosec` и `govulncheck` чистые в CI (с #nosec комментариями на 3 обоснованных false-positive)
- [x] Документация по безопасности добавлена в репозиторий (Day 19)
- [x] `docs/runbook.md` описывает ротацию секретов и реакцию на инциденты (webhook 8 ч, 152-ФЗ запросы)
- [ ] HTTPS на публичных endpoint'ах — настраивается ingress'ом стенда (Caddy/Nginx), не задача репо
- [ ] резервное видео демо — записывается перед презентацией, не коммитим в репо
