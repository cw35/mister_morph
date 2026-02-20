# Mistermorph Console SPA

This directory contains the Console SPA used by:

```bash
mistermorph console serve
```

Stack:
- Vue 3 + Vue Router
- `quail-ui`
- Vite (`src` -> `dist`)
- package manager: `pnpm`

## Runtime Notes

- Console APIs are served under `/console/api`.
- Console task views read from the daemon endpoint configured by `server.url` (default `http://127.0.0.1:8787`).
- Console itself does not persist task history.

## Features

- Overview:
  - grouped cards (basic, model, channels, runtime)
  - current LLM provider/model
  - channel configured/running state (Telegram/Slack dot badges)
  - auto-refresh every 60 seconds
- Tasks:
  - list + detail (read-only)
- TODO files:
  - edit `TODO.md` and `TODO.DONE.md`
- Contacts files:
  - edit `ACTIVE.md` and `INACTIVE.md`
- Persona files:
  - edit `IDENTITY.md` and `SOUL.md`
- Audit:
  - browse Guard audit files
  - windowed reads for large JSONL logs (`max_bytes` + `before`)
  - newest entries shown first in the UI
- Settings:
  - config snapshot + diagnostics
  - language selector
  - logout button (danger style)
- i18n:
  - English, Chinese, Japanese
  - language selector appears on Login and Settings (not in top nav)

## API Surface (under `/console/api`)

Auth:
- `POST /auth/login`
- `POST /auth/logout`
- `GET /auth/me`

Dashboard/system:
- `GET /dashboard/overview`
- `GET /system/health`
- `GET /system/config`
- `GET /system/diagnostics`

Tasks:
- `GET /tasks`
- `GET /tasks/{id}`

TODO files:
- `GET /todo/files`
- `GET /todo/files/{name}` (`TODO.md|TODO.DONE.md`)
- `PUT /todo/files/{name}`

Contacts files:
- `GET /contacts/files`
- `GET /contacts/files/{name}` (`ACTIVE.md|INACTIVE.md`)
- `PUT /contacts/files/{name}`

Persona files:
- `GET /persona/files`
- `GET /persona/files/{name}` (`IDENTITY.md|SOUL.md`)
- `PUT /persona/files/{name}`

Audit:
- `GET /audit/files`
- `GET /audit/logs?file=<name>&max_bytes=<n>&before=<offset>`

## Security and Caching Notes

- Console password is required (`console.password` or `console.password_hash`).
- Protected APIs require Bearer token auth.
- Anti-bruteforce protection is enabled in the backend.
- JSON API responses use no-store cache headers.
- SPA fetch requests use `cache: "no-store"`.

## Build (production static)

1. Build frontend:

```bash
cd web/console
pnpm install
pnpm build
```

2. Start daemon (task API source):

```bash
MISTER_MORPH_SERVER_AUTH_TOKEN=dev-token \
go run ./cmd/mistermorph serve --server-auth-token dev-token
```

3. Start console backend + static hosting:

```bash
MISTER_MORPH_CONSOLE_PASSWORD=secret \
MISTER_MORPH_SERVER_AUTH_TOKEN=dev-token \
go run ./cmd/mistermorph console serve --console-static-dir ./web/console/dist
```

4. Open:

`http://127.0.0.1:9080/console`

## Dev (hot reload)

1. Start daemon:

```bash
MISTER_MORPH_SERVER_AUTH_TOKEN=dev-token \
go run ./cmd/mistermorph serve --server-auth-token dev-token
```

2. Start console backend:

```bash
MISTER_MORPH_CONSOLE_PASSWORD=secret \
MISTER_MORPH_SERVER_AUTH_TOKEN=dev-token \
go run ./cmd/mistermorph console serve --console-static-dir ./web/console/dist
```

3. Start Vite dev server:

```bash
cd web/console
pnpm install
pnpm dev
```

4. Open:

`http://127.0.0.1:5173/console/`

Notes:
- Vite proxies `/console/api` to `http://127.0.0.1:9080`.
- During frontend dev, Vite page is enough; backend static `dist` is mainly for production serving.
