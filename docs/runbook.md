# Runbook — MAX University Event Bot

Шпаргалка эксплуатации для on-call / организатора стенда. Покрывает
ежедневные операции, наиболее частые инциденты и реакцию на запросы
пользователей по 152-ФЗ.

> Связанные документы: README.md (обзор), [`demo_walkthrough.md`](./demo_walkthrough.md)
> (продуктовые сценарии) и [`deploy.md`](./deploy.md) (развёртывание и продовый деплой).

---

## 1. Запуск, остановка, логи

```bash
# Поднять весь стек (postgres + migrate + bot + web).
make docker-up

# Остановить (volume pgdata сохраняется).
make docker-down

# Полная очистка с удалением БД (на демо после прогона).
docker compose -f deployments/docker-compose.yml down -v

# Только бот (после кода/конфига).
docker compose -f deployments/docker-compose.yml up -d --build bot

# Логи (stdout JSON slog, формат `LOG_FORMAT=json|text`).
docker compose -f deployments/docker-compose.yml logs -f bot
docker compose -f deployments/docker-compose.yml logs -f web
docker compose -f deployments/docker-compose.yml logs migrate   # одноразовая
```

Все логи бота — JSON-строки с полем `time`, `level`, `msg`, `service`.
PII в логах маскируется через `internal/pkg/secret.Mask*`. Cookie и токены не логируются.

Healthcheck:

```bash
curl -fsS http://localhost:8080/healthz   # bot
curl -fsS http://localhost:8081/api/healthz   # admin API
```

---

## 2. Конфигурация и роли

Все настройки — через `.env` (см. `.env.example`). Бот валидирует
конфиг при старте и падает с понятной ошибкой, если что-то критичное
отсутствует (`internal/app/config.go::Validate`).

### Добавить organizer'а

1. Узнать `max_user_id` пользователя (можно через бота — он логирует
   `user_id` в каждом сообщении уровня `debug`).
2. Дописать в `.env`:
   ```
   ORGANIZER_USER_IDS=123456789,987654321
   ```
   Значения — CSV `max_user_id`, без пробелов.
3. Рестарт бота:
   ```bash
   docker compose -f deployments/docker-compose.yml restart bot
   ```
4. Проверить: пользователь делает `/organizer` в боте, должен увидеть
   меню организатора. Если нет — посмотреть лог
   `"organizer access denied" user_id=...`.

`ADMIN_USER_IDS` — аналогично, для роли admin (на MVP она используется
только как «суперроль» над organizer; отдельных команд /admin нет).

### Проверить, что напоминания о регистрации живы

Напоминания хранятся в таблице `notifications` и создаются scheduler'ом
по `DEFAULT_EVENT_REMINDER_HOURS` (по умолчанию `24,1`).

Быстрые проверки:

```sql
-- Какие reminder'ы вообще есть в системе.
SELECT type, status, count(*)
FROM notifications
GROUP BY type, status
ORDER BY type, status;

-- Что запланировано по конкретному пользователю / событию.
SELECT id, type, status, scheduled_at, sent_at, error
FROM notifications
WHERE user_id = $1 AND event_id = $2
ORDER BY scheduled_at DESC;
```

Что важно помнить:

- если до старта события уже меньше 24 часов, `reminder_24h` не появится;
- если до старта уже меньше часа, scheduler не создаёт просроченный `reminder_1h`;
- dispatch-job отправляет pending-уведомления раз в минуту.

### Включить/выключить AI

- `GIGACHAT_AUTH_KEY=""` — выключает GigaChat-клиент целиком.
- `AI_EVENT_RECOMMENDER_ENABLED`, `AI_NOTIFICATION_REWRITER_ENABLED`,
  `AI_ORGANIZER_SUMMARY_ENABLED` — точечные флаги для трёх AI-сценариев.
- `GIGACHAT_INSECURE_TLS=true` — **только dev**, никогда в prod.

---

## 3. 152-ФЗ и команда `/forget_me`

Пользователь имеет право в любой момент удалить все свои данные:

1. В боте отправляет `/forget_me`.
2. Бот показывает предупреждение со списком того, что будет удалено
   (записи, история действий, согласие, FSM-state).
3. На «Подтвердить удаление» сервис вызывает `repo.Users.ForgetMe(ctx, userID)`,
   который в одной транзакции:
   - удаляет registrations пользователя (CASCADE снимает FK);
   - чистит `user_states`, `action_logs` (через `ON DELETE SET NULL`/CASCADE);
   - удаляет сам user-row.
4. Бот отвечает финальным сообщением и сбрасывает FSM (`/start` создаст
   нового user'а с нуля, без согласия).

### Если запрос пришёл оператору (письмо, телефон)

- Не пытайтесь удалять данные через psql вручную. Попросите пользователя
  открыть бота и нажать `/forget_me` — это самый чистый путь и не
  оставляет вырожденных FK.
- Если по каким-то причинам пользователь не может зайти в бота
  (заблокировали аккаунт MAX) — операция всё ещё делается одной командой
  на бэкенде. Для новых регистраций контакт уже не собирается, поэтому
  ориентируемся в первую очередь на `max_user_id` и `full_name_snapshot`;
  `contact_snapshot` может быть заполнен только у legacy-записей. После
  идентификации выполнить через psql на host'е базы:

  ```sql
  -- ВНИМАНИЕ: проверь user_id перед запуском. Откатить нельзя.
  BEGIN;
  SELECT id, max_user_id, full_name FROM users WHERE max_user_id = <ID>;
  DELETE FROM users WHERE max_user_id = <ID>;
  COMMIT;
  ```

  Каскадное удаление в БД дополнительно зачистит зависимые таблицы.
  Зафиксировать факт удаления и запрос в журнале обращений (вне репо).

---

## 4. Webhook режим: MAX отписал бота через 8 ч

Симптом — в логах:
```
"ensure subscription failed (continue)" err=...
"webhook decode failed" err=...
```
или просто отсутствие апдейтов в логах при попытках написать боту.

Что делать:

1. Проверить логи бота: ищем строку `"ensure subscription failed"`.
2. Убедиться, что `MAX_BOT_WEBHOOK_URL` доступен снаружи (curl с
   внешнего хоста; ingress/Caddy работает; TLS-сертификат не протух).
3. Рестартануть бота:
   ```bash
   docker compose -f deployments/docker-compose.yml restart bot
   ```
   На старте `App.ensureSubscription` сначала вызовет
   `GetSubscriptions`. Если нашего URL нет — `Subscribe(url, secret,
   update_types=[bot_started, message_created, message_callback])`.
4. Если рестарт не помогает (упорно 4xx от MAX API) — временно
   переключиться на long-poll: `MAX_BOT_MODE=longpoll` в `.env` + рестарт.
5. Параллельно — проверить, что `MAX_BOT_WEBHOOK_SECRET` не сменился,
   иначе будет приходить 401 на наш `/webhook/max`.

Профилактика — на проде держать `MAX_BOT_MODE=webhook` за reverse-proxy
с keep-alive, чтобы не было 8-часовых простоев. На демо проще запускать
в long-poll, поскольку он не требует публичного URL.

---

## 5. Ротация секретов

### `ADMIN_SESSION_KEY`

Подпись magic-link и session JWT (HMAC). **Ротация инвалидирует все
выданные magic-link'и и активные cookie session_jwt** — все организаторы
должны будут заново зайти через `/admin_login`.

Алгоритм:

```bash
# 1) Сгенерировать новый ключ (≥ 32 байта base64).
openssl rand -base64 32 | tr -d '\n'

# 2) Заменить значение в .env (или в secret store).
# 3) Рестарт бота.
docker compose -f deployments/docker-compose.yml restart bot

# 4) Старые сессии перестанут проходить /api/auth/me → редирект на /auth/login.
```

Никогда не публиковать значение в коммитах, чатах, скриншотах.
В логах ключ маскируется в `Config.String()` (см. `config.go`).

### `GIGACHAT_AUTH_KEY`

Клиент кеширует access_token внутри процесса (TTL ≥ 60s buffer
до истечения). При смене ключа в `.env` нужен **рестарт бота** —
без рестарта старый client продолжит ходить со старыми
client credentials. Симптом — `401 Unauthorized` в логах
`gigachat: oauth failed`.

### `MAX_BOT_TOKEN`

Меняется только при компрометации. После замены — рестарт бота. В
webhook-режиме MAX автоматически перестанет принимать апдейты на
старый токен; в long-poll — `GetUpdates` отдаст 401.

### `MAX_BOT_WEBHOOK_SECRET`

При смене — нужно одновременно:

1. Поменять значение в `.env`.
2. Сделать `Subscribe` заново (бот сделает сам на старте).
3. Рестарт бота.

Иначе `webhook/server.go::ConstantTimeEqual` отбросит апдейты от MAX.

---

## 6. Частые операции psql

База лежит в контейнере `deployments-postgres-1`, volume `pgdata`.
Кредсы — `app/app/maxbot` (по умолчанию из `.env.example`).

```bash
# Открыть psql внутри контейнера.
docker compose -f deployments/docker-compose.yml exec postgres \
  psql -U app -d maxbot

# Сколько участников на событии (registered + waitlist).
SELECT status, count(*) FROM registrations
WHERE event_id = 1 GROUP BY status;

# Статус миграций.
docker compose -f deployments/docker-compose.yml run --rm migrate /app/migrate status

# Откатить последнюю миграцию (ОСТОРОЖНО, обычно не нужно).
docker compose -f deployments/docker-compose.yml run --rm migrate /app/migrate down
```

---

## 7. Восстановление после инцидента

1. **БД повреждена / нужен откат.** Volume `pgdata` лежит в Docker
   volume. На демо-стенде проще сделать `make docker-down -v` и заново
   `make docker-up` — миграции и сид пересоздадутся.
2. **QR-картинка не доставлена.** В `registrations` есть
   `qr_sent_message_id`. Если оно `NULL`, пользователь может в боте
   нажать «Мои записи → Показать мой QR» — бот перегенерирует картинку из
   `attendance_code`.
3. **Рассылка или напоминания зависли.** Проверить `notifications.status='pending'`:
   ```
   SELECT status, count(*) FROM notifications
   WHERE event_id = $1 GROUP BY status;
   ```
   Scheduler-job `dispatchDue` запускается раз в минуту и обрабатывает
   до `NOTIFICATION_BATCH_SIZE=50` уведомлений за тик с
   `NOTIFICATION_RATE_LIMIT_RPS=20`. Если есть `failed` — посмотреть
   колонку `error` в той же таблице.
4. **Бот лупится с `ping max api: 401`.** Неверный `MAX_BOT_TOKEN`.
   Это поведение by design — бот не должен стартовать с битым токеном.
   Поправить `.env` → рестарт.

---

## 8. Фильтры мероприятий

Фильтры хранятся в FSM-контексте пользователя (таблица `user_states`, поле `context` JSONB).
Состояние сбрасывается при `/start` или `reset` FSM.

Доступные измерения:

| Параметр | Значения | JSON-поле в FSM |
|---|---|---|
| Формат | `offline` / `online` / `hybrid` / `""` | `event_filter` |
| Время | `today` / `week` / `""` | `event_time_filter` |
| Места | `true` / `false` | `event_seats_only` |
| Тема | `it` / `карьера` / `хакатон` / `поступление` / `олимпиада` / `devops` / `""` | `event_tag_filter` |

При активных фильтрах бот загружает до 200 ближайших открытых событий и фильтрует на стороне Go.
При отсутствии фильтров — прямой SQL-запрос с LIMIT/OFFSET (быстрее).

Если пользователь жалуется, что не видит событие в списке:
1. Убедиться, что у события `status = 'open'` и `starts_at > NOW()`.
2. Проверить активные фильтры — попросить нажать «Фильтры → Сбросить все».
3. Событие с `starts_at` в прошлом больше не показывается (даже если `status = 'open'`).

---

## 9. Удаление мероприятий

Удаление доступно только владельцу события (organizer) или admin.

**Через веб-админку:**

1. Войти в `/events/[id]`.
2. Прокрутить вниз до карточки «Опасная зона».
3. Нажать «Удалить мероприятие» → ввести название для подтверждения → «Подтвердить удаление».
4. После успеха — редирект на `/events`.

**Через API (curl):**

```bash
curl -X DELETE https://zhaba1337.eu.cc/api/events/{id} \
  -H "Cookie: sid=<session_jwt>"
```

Ответ `204 No Content` — удаление прошло. `403` — нет прав. `404` — событие не найдено.

> **Внимание:** удаление каскадно удаляет регистрации через FK ON DELETE CASCADE.
> Зарегистрированные участники теряют записи без уведомлений — используйте только
> если мероприятие отменяется (предварительно сделайте рассылку отмены через `/organizer`).

---

## 10. Ограничение регистрации по времени

Начиная с текущей версии:

- Мероприятие **пропадает из списка** сразу при `starts_at <= NOW()` (SQL: `AND starts_at > NOW()`).
- Даже если пользователь открыл карточку по deep-link до старта — попытка записаться после старта даёт ошибку «Мероприятие уже недоступно».
- `status = 'closed'` по-прежнему работает для ручного закрытия до старта.

Для мероприятий в прошлом организатор может:
- смотреть статистику через `/organizer → Статистика`
- делать check-in через `/checkin` (окно до starts_at+4h)
- но **не** открывать заново регистрацию (переход в `open` не вернёт событие в список, если starts_at уже прошёл)

---

## 11. Что НЕ делать

- Не редактировать БД напрямую (`UPDATE registrations SET status=...`)
  в обход сервисного слоя — потеряются логи действий и не сработает
  auto-promote из waitlist.
- Не коммитить `.env`, `.env.local`, дампы БД, скриншоты с QR-кодами.
- Не убирать `cap_drop: ALL` / `no-new-privileges` / `tmpfs` из
  docker-compose «для удобства» — это часть базовой harden'ой конфигурации
  продового контура.
- Не выключать `WAITLIST_ENABLED` посреди открытой регистрации —
  пользователи, попавшие в waitlist, перестанут промоутиться.
