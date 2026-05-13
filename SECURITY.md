# Security Policy

## Поддерживаемые версии

| Версия | Поддержка |
|---|---|
| 1.x | да |

## Контакт

Сообщения об уязвимостях: **security@example.com**. Время ответа — 48 часов.
В сообщении укажите: версия, шаги воспроизведения, потенциальное воздействие.

---

## Применённые меры

### Бот (MAX SDK + long-poll / webhook)

- TLS 1.2+ на webhook, валидный сертификат, `Authorization` header не логируется.
- Webhook secret проверяется через `crypto/subtle.ConstantTimeCompare`
  (см. `internal/pkg/secret/secret.go::ConstantTimeEqual`).
- Лимит тела webhook 1 MiB через `http.MaxBytesReader`.
- Retry на 429/5xx и сетевые ошибки с экспоненциальным backoff + jitter
  (`internal/pkg/retry/retry.go`).
- `recover()` в каждом handler dispatcher'а — паника не валит бота
  (`internal/bot/dispatcher.go`).
- Опасные действия (отмена записи, /forget_me, закрытие регистрации) —
  двухшаговое подтверждение с FSM-guard'ом (state + объект совпадают).
- RBAC: `service.Role.RequireEventOwner` на каждом организаторском хендлере.
  Admin может всё; organizer — только свои события (по `created_by`).
- Параметризованные SQL-запросы через `pgx/v5` (никаких string-форматов).
- Транзакция `RepeatableRead` + `SELECT ... FOR UPDATE` на event row
  для регистрации и check-in (защита от гонок capacity).
- PII (ФИО/email/телефон) маскируются:
  - в логах — НЕ пишем ФИО/email/телефон ни на каком уровне;
  - на UI организатора — `maskFullName` (Иванов И.) и `maskContact` (a***@example.com).
- 152-ФЗ: первая запись требует явного согласия (`reg_consent` FSM state),
  фиксируется `users.consent_at` + `consent_policy_ver`.
  Команда `/forget_me` физически удаляет пользователя (CASCADE → registrations,
  user_states; action_logs SET NULL для сохранения статистики).

### Веб-админка (REST API, без фронта на текущий момент)

- Аутентификация — magic-link JWT из бота (HS256, TTL 5 мин, `purpose=magic`).
- Session JWT (HS256, TTL 12 ч, `purpose=session`) только в
  `Set-Cookie: sid; HttpOnly; Secure; SameSite=Strict; Path=/`.
- JS никогда не видит токен (нет localStorage).
- На каждом запросе роль из БД сверяется с claims — расхождение даёт 401
  `role_changed` (revoke через смену роли работает мгновенно).
- CSRF: JSON-only API + `Origin/Referer` guard + `SameSite=Strict`.
- Security headers:
  - `Strict-Transport-Security: max-age=31536000; includeSubDomains`
  - `X-Frame-Options: DENY`
  - `X-Content-Type-Options: nosniff`
  - `Referrer-Policy: no-referrer`
  - `Permissions-Policy: camera=(self), microphone=(), geolocation=()`
- CORS только из `ADMIN_WEB_BASE_URL` с `credentials: include`.
- Логи без query-string и body (PII).
- PII в DTO маскирована по умолчанию (`full_name_masked`, `contact_masked`).

### QR check-in

- `attendance_code` = UUID v4 hex (32 char, 128 бит энтропии) — не predictable.
- Префикс `MAXUEB:` отсекает чужие QR-коды (любой другой QR → 400 без обработки).
- Бэкенд проверяет соответствие `event_id` из QR-payload и `registrations.event_id`
  (защита от подделки QR).
- Окно check-in: `[starts_at - 2h, ends_at + 4h]` — вне его → 409.
- Транзакция `SELECT ... FOR UPDATE` защищает от race-condition при
  одновременных сканах двумя сотрудниками.
- На повторный скан того же QR — `already_done: true`, не ошибка.
- Каждый успешный скан — `action_logs.checkin_scanned` (без PII).

### Контейнер и supply chain

- В Dockerfile (deployments/Dockerfile, появится с docker-compose): `USER app`
  (non-root), `read_only: true`, `cap_drop: ALL`, `no-new-privileges: true`,
  `tmpfs: /tmp` для QR-картинок.
- `go.sum` коммитим, CI запускает `govulncheck ./...` и `gosec ./...`.
- Все зависимости фиксированы в `go.mod` (без `@latest` в коде).
- `golangci-lint` собирается из исходников с текущей Go (не подцепляется
  бинарь с старой версией Go).

### CI

- 4 джобы (build+race tests, gofmt+vet+lint, govulncheck+gosec, migrate-on-postgres).
- Migrate-джоба поднимает Postgres 16 service и проверяет round-trip down/up.
- Pin Go = `stable` (избегаем CVE в старых версиях Go-stdlib).

---

## Процедура ротации секретов

### MAX bot token (`MAX_BOT_TOKEN`)

1. Кабинет MAX → Чат-боты → Интеграция → «Перевыпустить токен».
2. Обновить `MAX_BOT_TOKEN` в Docker secret / k8s Secret / .env.
3. `docker compose restart bot` (или `kubectl rollout restart`).
4. Если был webhook — перевыпустить подписку (на старте `ensureSubscription`).

### Webhook secret (`MAX_BOT_WEBHOOK_SECRET`)

1. Сгенерировать новый: `openssl rand -base64 48 | tr -d '/+' | cut -c1-48`.
2. `api.Subscriptions.Unsubscribe(ctx, currentURL)`.
3. Обновить env.
4. Рестарт бота, `ensureSubscription` подпишет заново.

### GigaChat AUTH_KEY (`GIGACHAT_AUTH_KEY`)

1. Личный кабинет developers.sber.ru → проект → «Перевыпустить ключ».
2. Обновить env. Старый протухает сразу.
3. Рестарт бота. Внутренний кеш `access_token` инвалидируется.

### Пароль БД (`DATABASE_URL`)

1. `ALTER USER app WITH PASSWORD '...'` (новый рандомный).
2. Обновить `DATABASE_URL`.
3. Rolling restart бота.

### `ADMIN_SESSION_KEY` (HMAC-ключ для magic и session JWT)

1. `openssl rand -base64 32` → новый ключ.
2. **Все живые JWT (magic + session) становятся недействительными** — HMAC
   не совпадёт. Это и есть единственный способ revoke в stateless-схеме.
3. Rolling restart бэкенда.
4. Все организаторы должны заново сделать `/admin_login` в боте.

> Таблиц `admin_sessions` / `login_tokens` нет — auth полностью stateless.

---

## Удаление данных пользователя (152-ФЗ)

1. Пользователь отправляет `/forget_me` в боте.
2. Бот показывает подтверждение (двухшаговое, с FSM-guard'ом).
3. При подтверждении — `DELETE FROM users WHERE id = ?` (каскадом удаляются
   registrations, user_states; action_logs SET NULL).
4. Action log `forget_me` пишется **ПЕРЕД** удалением, чтобы факт удаления
   сохранился в audit.
5. Удаление асинхронно реплицируется в бэкапы по их штатной ротации (30 дней
   по умолчанию — настраивается на уровне инфраструктуры).

---

## Реакция на инцидент

1. **Изолировать**: `docker compose stop bot` (либо scale=0 в k8s).
2. **Снять снапшот**: `pg_dump` + tar логов.
3. **Отозвать все секреты** (см. процедуры выше) — особенно
   `ADMIN_SESSION_KEY` (это аннулирует все JWT мгновенно).
4. **Деплой с нуля** на чистый хост с новыми секретами.
5. **Постмортем** в `docs/incidents/YYYY-MM-DD-<slug>.md` в течение 7 дней:
   - что произошло, когда, как обнаружили;
   - root cause;
   - что починили технически;
   - что меняется в процессах.

---

## Известные ограничения и trade-offs

### Не реализовано (откладывается на post-MVP)

- **Rate-limit per-user** (token bucket) — пока ловится на уровне MAX SDK
  ретраев (429). День 17 — добавим явный per-user bucket.
- **Webhook режим** — пока только long-polling. День 18.
- **Scheduler reminders** — `service.Notification.ScheduleUpcomingReminders`
  и `DispatchDue` — stub'ы. День 16.
- **AI-сервисы** (GigaChat) — заглушки в handlers. День 16.
- **Frontend Next.js** — пользователь сознательно отложил; REST API готов
  и валидируется через curl/Postman.

### Trade-offs

- **CSV-экспорт в чат-бот** обрезается на 3500 символов — для полного экспорта
  предусмотрен `/api/events/:id/participants` (Стрим CSV через web-админку
  будет в Дне 14).
- **Auto-promote из waitlist** (без подтверждения пользователем) —
  упрощение MVP. План §23 День 8 допускает both, мы выбрали auto для
  снижения нагрузки на UX (пользователь и так получит уведомление о новой
  записи через Day 16 reminders, если они будут включены).
- **Маскировка контактов** работает по простой эвристике (первые 2 + ***+ последние 2).
  Для полного PII-compliance в админке нужен «Раскрыть» с отдельным audit-log'ом —
  планируется на День 13 P1.

---

## Что находится вне scope

- DDoS-защита транспортного уровня — на edge (Cloudflare / антиDDoS провайдера).
- Бэкапы БД и их шифрование — ответственность платформы хостинга.
- Аудит инфраструктуры (хост, оркестратор) — отдельный документ ops-команды.
- Penetration testing — рекомендуем перед прод-релизом провести внешний pen-test.
