# MAX Bot Admin (web)

Next.js 14 admin для университетского MAX-бота. Авторизация — magic-link
из бота (`/admin_login` → `/auth?t=<jwt>`), session-cookie HttpOnly.

## Stack

- Next.js 14 (App Router, standalone output)
- React 18 + TypeScript
- TailwindCSS (минимальный дизайн-токен, без shadcn CLI)
- `@yudiel/react-qr-scanner` для check-in
- API-вызовы — обычный `fetch` через Next.js rewrite в Go-бэкенд

## Локальный запуск

```bash
cd web
npm install
API_UPSTREAM=http://localhost:8081 npm run dev
# открыть http://localhost:3000
```

## Production

См. `deployments/docker-compose.yml`, сервис `web`.

## Структура

- `app/` — App Router pages
  - `auth/` — обмен magic-токена на session-cookie
  - `(authenticated)/` — авторизованная зона (общий layout с Nav)
    - `dashboard/`, `events/`, `events/[id]/`,
      `events/[id]/participants/`, `events/[id]/broadcast/`, `checkin/`
- `components/ui/` — Button / Card / Input / Textarea / Badge
- `components/nav.tsx` — верхняя навигация + logout
- `lib/api.ts` — типизированный fetch с `HttpError`
- `lib/types.ts` — DTO, отзеркаленные с `internal/transport/adminapi`
- `lib/format.ts` — даты/статусы
