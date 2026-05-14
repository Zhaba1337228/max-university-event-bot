---
name: testing-admin-web
description: Test the Next.js admin web of max-university-event-bot end-to-end without a real MAX_BOT_TOKEN. Use when verifying auth/RBAC, check-in scanner (camera UX + manual QR input, encrypted MAXUEB1 + legacy MAXUEB:), or any UI feature that does not require a live MAX/GigaChat integration.
---

# Testing the admin web (max-university-event-bot)

## When to use
Whenever you need to exercise the admin web UI end-to-end on a VM without hardware camera and without a real MAX bot token. Covers auth/RBAC, check-in scanner, encrypted/legacy QR payloads, and DB-state assertions.

## Devin Secrets Needed
None required for local testing. The repo seeds three users by role IDs from `.env`:
- `ADMIN_USER_IDS=999998`
- `ORGANIZER_USER_IDS=999999`
- `STAFF_USER_IDS=999997`
The `ADMIN_SESSION_KEY` in `.env.example` (`devdevdev...dev1`) is a local-only dev key and is fine to use directly.

## Stack setup
```bash
cd repos/max-university-event-bot/deployments
docker compose up -d --build postgres migrate bot web
docker compose ps    # all should be healthy
```
- Web: http://localhost:3000
- Admin API: http://localhost:8081 (mounted inside bot container, requires `Authorization: Bearer <session_jwt>` cookie via web)
- Postgres: localhost:5432 (`app`/`app`/`maxbot`)

If you change Go code that the bot or admin API runs, you MUST rebuild the bot image: `docker compose build bot && docker compose up -d bot`. The web container also needs rebuild on `web/` changes. Containers do NOT auto-pick-up source changes.

`MAX_BOT_DEV_SKIP_PING=true` (in `.env.example`) lets the bot start without a valid `MAX_BOT_TOKEN` — admin API still serves requests.

## Magic-link login per role
The admin web uses magic-link auth. Generate links via `cmd/devmagic` (dev-only CLI):
```bash
cd repos/max-university-event-bot
ADMIN_SESSION_KEY=$(grep -E '^ADMIN_SESSION_KEY=' .env | cut -d= -f2) \
DATABASE_URL='postgres://app:app@localhost:5432/maxbot?sslmode=disable' \
ADMIN_WEB_BASE_URL='http://localhost:3000' \
go run ./cmd/devmagic <users.id>     # 1=admin, 2=organizer, 3=staff (seed defaults)
```
The output URL is single-use, TTL 5 minutes. **Magic JWTs are long; never type them by hand — always paste via xclip:**
```bash
echo "$LINK" | xclip -selection clipboard
```
Then in the browser: click address bar → Ctrl+A → Ctrl+V → Enter. Typing the URL through `xdotool` often drops a digit in the `exp` claim and breaks the JWT.

Session cookie after exchange (`/api/auth/exchange`) is 24h, so one magic-link per role per session is enough.

## Common DB operations
Registration reset (for repeatable check-in tests):
```bash
docker exec deployments-postgres-1 psql -U app -d maxbot -c \
  "UPDATE registrations SET status='registered', checkin_at=NULL, checkin_by=NULL WHERE id=1;"
```
Verify state:
```bash
docker exec deployments-postgres-1 psql -U app -d maxbot -c \
  "SELECT id,status,checkin_by,checkin_at FROM registrations WHERE id=1;"
```
Event check-in window: `events.starts_at <= now() <= events.ends_at` must hold; if it slips out of window (because seeds are time-relative), bump it:
```bash
docker exec deployments-postgres-1 psql -U app -d maxbot -c \
  "UPDATE events SET starts_at = now() - interval '30 min', ends_at = now() + interval '2 hours' WHERE id=1;"
```

## Check-in: testing QR payloads (encrypted + legacy)
The scanner accepts BOTH formats:
- New encrypted: `MAXUEB1.<base64url(nonce|ciphertext|gcm-tag)>`
- Legacy plaintext: `MAXUEB:<event_id>:<32-char attendance_code>`

Generate test payloads locally (helper must live inside the module tree because it uses `internal/service`; create temporarily, delete after):
```bash
mkdir -p internal/service/qrgen_tmp && cat > internal/service/qrgen_tmp/main.go <<'EOF'
package main

import (
	"fmt"; "os"; "strings"; "time"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

func main() {
	key := os.Getenv("ADMIN_SESSION_KEY")
	qr, _ := service.NewQR(key)
	code := strings.Repeat("a", 32)
	good := qr.BuildQRPayload(1, code)
	fmt.Println("GOOD:", good)
	fmt.Println("EXPIRED:", qr.BuildQRPayloadWithTTL(1, code, -time.Hour))
	last := good[len(good)-1]; var nl byte; if last=='A'{nl='B'}else{nl='A'}
	fmt.Println("TAMPERED:", good[:len(good)-1]+string(nl))
	fmt.Println("LEGACY: MAXUEB:1:"+code)
}
EOF
ADMIN_SESSION_KEY=$(grep -E '^ADMIN_SESSION_KEY=' .env | cut -d= -f2) \
  go run ./internal/service/qrgen_tmp/main.go
rm -rf internal/service/qrgen_tmp
```
IMPORTANT: the helper MUST use the same `ADMIN_SESSION_KEY` value that the running bot container has. If not, every encrypted payload will be rejected as `qr_tampered` (auth-tag won't match — the AES key is derived via `SHA-256(ADMIN_SESSION_KEY)`).

Expected error mapping on `POST /api/checkin`:
- valid → 200 `{ok: true, ...participant...}`
- tampered (auth-tag fails) → 400 `{error: "qr_tampered"}` → UI: "QR подделан или сгенерирован чужим сервером"
- expired (TTL exceeded) → 400 `{error: "qr_expired"}` → UI: "Срок действия QR истёк"
- bad format → 400 `{error: "qr_invalid"}` → UI: "Некорректный QR-код"
- unknown reg → 400 `{error: "registration_not_found"}` → UI: "Регистрация не найдена или неактивна"
- organizer role (not staff/admin) → 403 `{error: "role_forbidden"}`

## Camera UX on VM (no hardware camera)
The VM has no `/dev/video0`, so `navigator.mediaDevices.getUserMedia` will fail with `NotFoundError` or `NotAllowedError`. The page MUST:
1. NOT call `getUserMedia` on mount — only on explicit click of "Включить камеру".
2. Show a placeholder with **all 4 corner brackets** (top-left, top-right, bottom-left, bottom-right) before grant. If only 2 corners show, the container is missing `aspect-square` and the SVG overlay is clipping.
3. After click → state `denied` with text + "Повторить запрос" button.

To test camera UX with real hardware camera, use a real desktop browser session — not the VM.

## RBAC quick-reference
- `admin` (users.id=1): sees Дашборд / Мероприятия / Check-in
- `organizer` (users.id=2): sees Дашборд / Мероприятия; `/checkin` → 403 redirect to `/forbidden?reason=checkin_organizer`
- `staff` (users.id=3): sees ONLY Check-in (no Дашборд or Мероприятия)

## Recording GUI tests
Before starting `recording_start`, maximize the window:
```bash
sudo apt-get install -y wmctrl 2>/dev/null
wmctrl -r :ACTIVE: -b add,maximized_vert,maximized_horz
```
Do NOT use `xdotool key super+Up` — it tiles to half-screen on many WMs.

Use `annotate_recording` aggressively: one `test_start` per scenario, one `assertion` per outcome. Reset DB between mutating tests so the recording shows clean before/after.

If the VM restarts mid-recording, the in-flight recording is lost — re-run the flow and start a new recording. Save key screenshots after each major UI state with `computer(action=act, actions=[{action: screenshot}])` so you have evidence even if the recording dies.

## CI on this repo
Required checks (all must pass): `gofmt + go vet + golangci-lint`, `build + test (race)`, `migrate up against real Postgres`, `web (next build)`.

Non-required (`continue-on-error`): `govulncheck + gosec`. As of mid-2026 there are 5 known preexisting false-positives that fail this check on `main` too:
- G404 jitter in `internal/pkg/retry/retry.go`
- G402 InsecureSkipVerify in `internal/external/gigachat/client.go`
- G101 sample connection-string in `cmd/migrate/main.go`
- G118 background context in `internal/transport/webhook/server.go`
- G118 background context in `internal/transport/adminapi/server.go`

If gosec fails on your PR with only these 5 findings → preexisting, not blocking. If it fails with anything new, fix it.

Workflow trigger: `branches: [main]` — CI only runs on PRs targeting `main`. If you base a branch on a non-main feature branch, CI won't trigger until you rebase / retarget.

## Files you'll typically inspect
- `internal/service/qr.go` — QR encryption (AES-256-GCM, new+legacy parsing, TTL)
- `internal/service/attendance.go` — check-in business logic
- `internal/transport/adminapi/handlers.go` — error-to-HTTP mapping
- `web/app/(authenticated)/checkin/page.tsx` — scanner page + camera state machine
- `web/middleware.ts` — server-side RBAC for routes
- `cmd/devmagic/main.go` — magic-link generator (TTL 5m)
