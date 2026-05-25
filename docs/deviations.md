# Deviations from initial plan

> Журнал расхождений между первоначальным планом и реальной реализацией.
> Каждое отклонение — отдельный абзац формата:
>
> 1. **Что было в плане** (раздел и пункт).
> 2. **Что в реальности** (что обнаружили в SDK/API/окружении).
> 3. **Почему** (источник правды).
> 4. **Дата** и день дорожной карты.

---

## День 1 (бутстрап репозитория)

### 1. Модуль Go

- **План:** `github.com/<org>/max-university-event-bot` (плейсхолдер).
- **Реальность:** `github.com/Zhaba1337228/max-university-event-bot`.
- **Почему:** под аккаунтом владельца репозитория на GitHub.
- **Дата:** 2026-05-13.

### 2. Go 1.25.4 вместо 1.24

- **План:** `go 1.24+` (см. раздел 3.1).
- **Реальность:** `go 1.25.4` (host toolchain).
- **Почему:** `1.24+` в плане означает «минимум», 1.25 совместима и используется в env.
- **Дата:** 2026-05-13.

---

## День 2 (тексты + клавиатуры + payloads)

### 3. MAX SDK: тип Keyboard вместо KeyboardBuilder

- **План:** раздел 16.2 — `func MainMenu(api *maxbot.Api) *maxbot.KeyboardBuilder`.
- **Реальность (SDK v1.6.17):** возвращаемый тип — `*maxbot.Keyboard`. Метод `api.Messages.NewKeyboardBuilder()` существует, но возвращает `*Keyboard`. Это просто naming — функционально совпадает.
- **Источник:** `keyboard.go` и `messages.go` в `github.com/max-messenger/max-bot-api-client-go@v1.6.17`.
- **Решение:** во всех `keyboards/*.go` используется `*maxbot.Keyboard`.
- **Дата:** 2026-05-13.

### 4. MAX SDK: Send возвращает только error

- **План:** раздел 11.5 — `_, err := api.Messages.Send(ctx, msg)`.
- **Реальность (SDK v1.6.17):** `Send(ctx, m *Message) error` (один возврат). Для получения отправленного `schemes.Message` — `SendWithResult(ctx, m) (*schemes.Message, error)`.
- **Источник:** `messages.go`.
- **Решение:** обычная отправка — `api.Messages.Send`. Когда нужен `message_id` (QR-картинка, см. День 15) — `SendWithResult`.
- **Дата:** 2026-05-13.

### 5. MAX SDK: Edit/Delete вместо EditMessage/DeleteMessage в плане

- **План:** раздел 11.11 — `Edit`, `Delete`.
- **Реальность:** методы называются `EditMessage(ctx, messageID, m)` и `DeleteMessage(ctx, messageID)`.
- **Источник:** `messages.go`.
- **Решение:** используем реальные имена.
- **Дата:** 2026-05-13.

### 6. Размер страницы списка событий — 8 вместо абстрактного

- **План:** раздел 16.2 — `func EventList(api, events)` без указания pageSize.
- **Реальность:** добавлена константа `keyboards.PageSize() == 8`. MAX-лимит 30 рядов на клавиатуру; 8 — комфортно для мобильного.
- **Решение:** при превышении страница ограничивается; навигация «Назад/Дальше» работает через offset.
- **Дата:** 2026-05-13.

### 7. Состояние ForgetMeConfirm добавлено в FSM

- **План:** раздел 14.1 — нет явного состояния для двухшагового /forget_me.
- **Реальность:** добавил `StateForgetMeConfirm`, чтобы handler мог проверить, что callback пришёл из ожидаемого экрана (защита от устаревших кнопок).
- **Решение:** при `/forget_me` сначала Save(ForgetMeConfirm), затем при ForgetMeYes → реальное удаление.
- **Дата:** 2026-05-13.

### 8. Добавлены MyShowQR/OrgAISummary/ConsentRecorded и др. payloads/messages

- **План:** базовый набор callback-payload'ов и шаблонов.
- **Реальность:** дополнительно сделаны конструкторы под кнопки и тексты, которые понадобятся в днях 15-16 (QR-показ, AI-сводка организатору, подтверждение согласия, MyRegistrationsList).
- **Решение:** объявлять заранее дешевле, чем потом редактировать публичный пакет.
- **Дата:** 2026-05-13.

---

## День 3 (миграции + репозитории)

### 9. Querier интерфейс вместо pgxpool.Pool в репозиториях

- **План:** раздел 10.2 — `type registrationsRepo struct{ pool *pgxpool.Pool }`.
- **Реальность:** репозитории stateless. Принимают `Querier` (универсальный интерфейс для pgxpool.Pool и pgx.Tx) **параметром каждого метода**.
- **Почему:**
  1. Один и тот же репозиторий вызывается и снаружи транзакции, и внутри неё (для `Register` с `SELECT FOR UPDATE`).
  2. pgxmock тоже реализует этот интерфейс — тесты пишутся без боли.
  3. Можно сразу подсадить тест на реальный pgx через testcontainers, без переписывания.
- **Стоимость:** в сигнатуре методов появился лишний параметр. Сервисы будут получать `*pgxpool.Pool` через DI и явно передавать.
- **Дата:** 2026-05-13.

### 10. Goose требует Go ≥ 1.25.7

- **План:** `go 1.24+`.
- **Реальность:** `github.com/pressly/goose/v3@v3.27.1` требует Go ≥ 1.25.7. После `go get` Go автоматически переключился на 1.25.7.
- **Решение:** `go.mod` ставит `go 1.25.7`. На CI/Dockerfile нужно использовать `golang:1.25-alpine` (минор-версия совпадает).
- **Дата:** 2026-05-13.

### 11. Миграции встроены через embed.FS

- **План:** раздел 21.4 — `goose.RunContext(ctx, cmd, db, "migrations")` (чтение с диска).
- **Реальность:** `migrations/embed.go` встраивает все *.sql через embed.FS; `cmd/migrate` использует `goose.SetBaseFS(migrations.FS)` и каталог `.`.
- **Почему:** утилита работает из любой директории и не зависит от копирования файлов в Docker. `migrate create` всё ещё пишет на диск (через временный сброс FS).
- **Дата:** 2026-05-13.

### 12. EventStats расширён полями Attended и NoShow

- **План:** `domain.EventStats` имеет Capacity/Registered/Cancelled/Waitlist/FreeSeats/TopInterests.
- **Реальность:** добавлены `Attended` и `NoShow` — нужны для дашборда организатора (День 10) и админки (День 13).
- **Дата:** 2026-05-13.

### 13. CHECK-constraints на enum-колонки

- **План:** раздел 8 — комментарии «-- role: applicant|organizer|admin».
- **Реальность:** добавлены `CHECK (role IN (...))` в миграциях users, events, registrations, notifications. Лишний слой защиты от опечаток в коде и ручных правок в БД.
- **Дата:** 2026-05-13.

---

## День 4 (long-poll, dispatcher, /start)

### 14. SDK: `WithApiTimeout` вместо `WithClientTimeout`

- **План:** раздел 11.2 — `maxbot.WithClientTimeout(30 * time.Second)`.
- **Реальность:** в SDK v1.6.17 есть `WithApiTimeout`, `WithPauseTimeout`, `WithHTTPClient`, но НЕТ `WithClientTimeout`.
- **Источник:** `options.go` в `github.com/max-messenger/max-bot-api-client-go@v1.6.17`.
- **Решение:** используем `WithApiTimeout` — это таймаут на сам long-poll цикл.
- **Дата:** 2026-05-13.

### 15. SDK: `Callback.CallbackID` (а не `CallbackId`)

- **План:** раздел 11.7 — `upd.Callback.CallbackId`.
- **Реальность:** в SDK поле названо `CallbackID` (с большой `D`). Любопытно, что соседние `UserId`, `ChatId` — с маленькой `d`. Мешанина в самом SDK, мы её повторяем для совместимости.
- **Источник:** `schemes/schemes.go`.
- **Решение:** в коде везде `CallbackID`.
- **Дата:** 2026-05-13.

### 16. CI: pin Go = `stable` вместо `1.25.x`

- **План:** не специфицировано.
- **Реальность:** govulncheck флагнул `GO-2026-4971` (Panic in net.Dial on Windows). Фикс в Go 1.25.10, а setup-go с `1.25.x` берёт latest minor, которая на момент CI run могла быть 1.25.9.
- **Решение:** в CI используем `go-version: stable` — всегда последняя стабильная.
- **Дата:** 2026-05-13.

### 17. Bot ctx в виде handlers.go вместо отдельного ctx.go

- **План:** раздел 15.1 — `internal/bot/ctx.go` с типом Ctx, который содержит api/log/services/fsm/update.
- **Реальность:** на день 4 такой агрегатор overkill — у каждого handler уже есть свои зависимости в полях. `handlers.go` (вместо `ctx.go`) держит корневой Handlers с маршрутизацией.
- **Решение:** когда (в дни 5+) появится много handlers — можно мигрировать на агрегатор. Сейчас — KISS.
- **Дата:** 2026-05-13.

### 18. На день 4 — только /start, /help и главное меню

- **План:** раздел 23 (День 4) — то же самое.
- **Реальность:** другие команды (`/organizer`, `/admin_login`, `/forget_me`) и callback-группы (`ev:`, `reg:`, `my:`, `wl:`, `cancel:`, `org:`, ...) определены как payloads, но handlers ещё не реализованы — будут в днях 5-12.
- **Дата:** 2026-05-13.

### 19. notifications_dedup: IMMUTABLE-функция вместо date_trunc / extract

- **План:** раздел 8.9 — `CREATE UNIQUE INDEX uniq_notif_dedup ON notifications (user_id, event_id, type, date_trunc('minute', scheduled_at))`.
- **Реальность (после двух итераций):**
  - `date_trunc('minute', timestamptz)` — STABLE (зависит от session timezone). Postgres отказывает.
  - `EXTRACT(EPOCH FROM timestamptz)` — тоже STABLE по той же причине.
  - `EXTRACT(EPOCH FROM timestamp)` — IMMUTABLE, но требует каста timestamptz → timestamp.
  - `timestamptz AT TIME ZONE 'UTC'` (с **literal** timezone) — IMMUTABLE и даёт timestamp без TZ.
- **Решение:** определена кастомная `notif_minute_bucket(timestamptz) RETURNS bigint LANGUAGE sql IMMUTABLE PARALLEL SAFE`, использующая `AT TIME ZONE 'UTC'` + `EXTRACT(EPOCH FROM timestamp)` + `/ 60`. Уникальный индекс и ON CONFLICT в `repo.Notifications.Schedule` ссылаются на эту функцию.
- **Источники:** CI runs #25824279837 и #25824557380.
- **Дата:** 2026-05-13.

### 20. CI: golangci-lint собираем через `go install` (Go 1.25 mismatch)

- **План:** не специфицировано.
- **Реальность:** официальный `golangci-lint-action@v6` ставит бинарь, скомпилированный с Go 1.24. С `go.mod = go 1.25.7` он валится: «the Go language version used to build golangci-lint is lower than the targeted Go version».
- **Решение:** в CI вызываем `go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8` (соберётся с установленной Go stable) и запускаем напрямую из `$(go env GOPATH)/bin`.
- **Дата:** 2026-05-13.
