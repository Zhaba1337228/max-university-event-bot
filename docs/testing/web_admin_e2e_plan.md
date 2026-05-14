# E2E test plan: web-admin UI (PR #2)

PR: https://github.com/Zhaba1337228/max-university-event-bot/pull/2 (merged)

## What changed (user-visible)

- `MAX_BOT_DEV_SKIP_PING=true` позволяет поднять admin API без живого `MAX_BOT_TOKEN`.
- `cmd/devmagic <users.id>` локально выдаёт magic-link.
- `web` контейнер теперь `(healthy)` (HEALTHCHECK на `127.0.0.1:3000/auth/login`).
- `/api/*` корректно проксируется на `bot:8081` (раньше промахивался в `localhost:8081` внутри web).

## Primary E2E flow

Один организатор (`users.id=2`, `max_user_id=999999`, role=organizer, owner всех 3 seed-событий) проходит полный путь:

1. `/auth?t=<magic>` → редирект на `/dashboard`
2. `/dashboard` → 3 свои события в списке + три stat-card с правильными цифрами
3. `/events` → таб «Мои» и «Все открытые» возвращают одно и то же (всё owned by user 2)
4. `/events/1` (День открытых дверей) → видна вместимость 100, статус open
5. Закрыть регистрацию → badge меняется на «Закрыта»
6. Снова открыть → badge возвращается в «Открыта»
7. `/events/1/participants` → в таблице есть один pre-seeded регистрант с маскированными ФИО/контактом
8. `/events/1/broadcast` → форма не даёт отправить пустое; принимает текст в пределах 4000 символов
9. `/checkin` → камера запрашивается; ручной ввод плохого QR возвращает понятную ошибку
10. Кнопка «Выйти» → редирект на `/auth/login`, на `/dashboard` после этого нельзя (редирект обратно)

Pre-seed (НЕ часть recording — делаем заранее через psql):

```sql
INSERT INTO registrations(event_id, user_id, full_name_snapshot, contact_snapshot, status, registered_at)
VALUES (1, 1, 'Иванов Иван Иванович', '+79991234567', 'registered', NOW());
```

## Test cases (adversarial)

Для каждого: «как бы это выглядело, если бы изменение было сломано».

### T1. Magic-link → session cookie → `/dashboard`

Файлы: `web/app/auth/page.tsx:13-62`, `internal/transport/adminapi/handlers.go:19-56`

| Шаг | Ожидаем | Если сломано |
| --- | --- | --- |
| Открыть `http://localhost:3000/auth?t=<valid-jwt>` | Внутри секунды: URL меняется на `/dashboard`, видна шапка «MAX Bot Admin», три stat-card | Остаётся на `/auth`, видна красная плашка «Не удалось войти» |

**Конкретные значения для PASS:**
- Network-вкладка показывает `POST /api/auth/exchange` → `204 No Content`
- Cookie `session_jwt=...` установлена с `HttpOnly`, `Secure`, `SameSite=Strict`
- `GET /api/auth/me` → 200 с `{"user":{"id":2,"role":"organizer"}}`

**Конкретные значения для FAIL (доказательство, что тест ловит регрешн):**
- Если API_UPSTREAM сломан: `POST /api/auth/exchange` → 502 или `ECONNREFUSED` в web-логах → красная плашка
- Если DEV_SKIP_PING сломан: docker `ps` показывает bot `(unhealthy)` / restarting → 502 на всех `/api/*`

### T2. Dashboard — мои события и статистика

Файлы: `web/app/(authenticated)/dashboard/page.tsx:11-94`, `internal/transport/adminapi/handlers.go:306-331`

| Шаг | Ожидаем |
| --- | --- |
| На `/dashboard` дождаться загрузки | Stat-card «Мои мероприятия» = **3**; «Зарегистрировано» = **1** (после pre-seed); «Предстоящие» = **3** (все даты в будущем) |
| Прокрутить ниже | В блоке «Мои мероприятия» три ссылки: «День открытых дверей ИТ-направлений», «Консультация по поступлению», «Онлайн-знакомство с направлением «Программная инженерия»». Каждая со статусом «Открыта». |

**FAIL-detector:** если статистика берётся не от моего user_id, цифры будут либо 0, либо больше 3 (если включит чужие).

### T3. Events list — таб «Мои» vs «Все открытые»

Файл: `web/app/(authenticated)/events/page.tsx:14-110`

| Шаг | Ожидаем |
| --- | --- |
| Перейти на `/events`, остаться на табе «Мои» | 3 события — те же, что на дашборде |
| Кликнуть «Все открытые» | Те же 3 события, плюс столбец «free_seats/capacity», например `100/100`, `30/30`, `200/200` (никто кроме T2-регистранта не записан, и он на event 1) |

**FAIL-detector:** если запрос идёт без `?status=mine`, на «Мои» появятся чужие события (но в seed их нет). Если `free_seats` не отображается на «Все открытые» — поломан DTO.

### T4. Event detail — открытие / закрытие регистрации

Файлы: `web/app/(authenticated)/events/[id]/page.tsx:13-186`, `internal/transport/adminapi/handlers.go:181-209`

| Шаг | Ожидаем |
| --- | --- |
| `/events/1` | Заголовок «День открытых дверей ИТ-направлений», badge «Открыта», stat-grid: Зарегистр.=1, Свободно=99, Вместим.=100 |
| Кликнуть кнопку «Закрыть регистрацию» | Toast «Регистрация закрыта.», badge меняется на «Закрыта», stat не меняются |
| Кликнуть «Открыть регистрацию» | Toast «Регистрация открыта.», badge снова «Открыта» |

**FAIL-detector:** если ownsEvent broken, на close/open вернётся 403 → видна красная плашка вместо toast. Если backend не обновил status, badge останется как был.

### T5. Participants — поиск + пагинация

Файл: `web/app/(authenticated)/events/[id]/participants/page.tsx:15-158`

| Шаг | Ожидаем |
| --- | --- |
| `/events/1/participants` | Заголовок «Всего: 1», одна строка в таблице с ФИО маскированным (например, «Иванов И.И.»), контакт маскированный («+7-***-***-45-67» или подобная маска) |
| В поле поиска ввести `Иванов`, кликнуть «Найти» | Та же одна строка остаётся |
| В поле поиска ввести `Несуществующий`, кликнуть «Найти» | Текст «Никого не найдено.» |

**FAIL-detector:** если поиск не работает на бэке (filter в Go), при «Несуществующий» останется та же строка.

### T6. Broadcast — валидация формы

Файлы: `web/app/(authenticated)/events/[id]/broadcast/page.tsx:14-99`, `internal/transport/adminapi/handlers.go:211-243`

| Шаг | Ожидаем |
| --- | --- |
| `/events/1/broadcast` | Textarea пустая, кнопка «Отправить» disabled |
| Ввести «Тест рассылки» | Кнопка «Отправить» active, счётчик «Осталось» = 4000−13 = 3987 |
| Кликнуть «Отправить», в confirm-диалоге нажать «Отмена» | Никакого `POST /api/events/1/broadcast` в network, форма не очищается |
| Кликнуть «Отправить», подтвердить | Зелёный текст «Доставлено: N» (N=0, потому что MAX API не работает в dev-режиме, но валидация прошла и backend ответил) |

**FAIL-detector:** если кнопка не disabled на пустую форму → можно отправить пустоту (backend вернёт 400, увидим красный текст). Если confirm пропускается → POST уйдёт без подтверждения.

**ВАЖНО для T6:** в DEV_SKIP_PING сценарии MAX API недоступен, `Notification.SendBroadcast` всё равно идёт в БД и пытается отослать; ожидаем либо 0 sent + 200, либо 500 «Не удалось отправить». Любой response — пасс, главное что валидация формы работает.

### T7. Check-in — ручной ввод bad QR

Файлы: `web/app/(authenticated)/checkin/page.tsx:22-176`, `internal/transport/adminapi/handlers.go:252-301`

| Шаг | Ожидаем |
| --- | --- |
| `/checkin` | Браузер показывает диалог «localhost:3000 запрашивает доступ к камере» (в headless-режиме скажет «Разрешите доступ» либо покажет чёрный квадрат) |
| Скроллить вниз до «Ввести вручную», вписать `garbage` | Кнопка «Отметить» active |
| Кликнуть «Отметить» | Внизу появляется карточка «Ошибка» с текстом «Некорректный QR-код» |
| Стереть и ввести `MAXUEB:1:fakecode123` | Снова «Ошибка», но текст «Регистрация не найдена или неактивна» |

**FAIL-detector:** если backend не различает форматы, оба плохих QR дадут одну и ту же ошибку (либо вообще нет ошибки). Если страница не загружается без камеры → не увидим форму ручного ввода.

### T8. Logout

Файл: `web/components/nav.tsx:13-63`

| Шаг | Ожидаем |
| --- | --- |
| На любой странице (например `/dashboard`) кликнуть кнопку «Выйти» в правом верхнем углу | Редирект на `/auth/login` |
| Вернуться на `/dashboard` напрямую через адресную строку | Layout видит 401 на `/api/auth/me` → редирект на `/auth/login` |

**FAIL-detector:** если cookie не очищается → `/dashboard` загрузится без перехода.

## Что НЕ тестируем (явно)

- Бот в MAX — `MAX_BOT_DEV_SKIP_PING=true`, реальные апдейты не приходят.
- Реальная отправка рассылки и check-in в участников MAX — тоже зависит от живого API.
- GigaChat-улучшение текста рассылки — на /broadcast UI нет AI-кнопки (не реализована для веб-админки).
- Регистрация на событие из веб-админки — фича только в боте, не в web UI.

## Артефакты

- Recording: один непрерывный, начинается на `/auth?t=...`, заканчивается на `/auth/login` после logout.
- Screenshots: pass/fail per test case в `docs/testing/screenshots/`.
- Test report: `docs/testing/web_admin_e2e_report.md` с итогом по каждому T1–T8.
- PR comment с резюме (см. PR #2).
