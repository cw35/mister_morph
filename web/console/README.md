# Mistermorph Console SPA

This directory contains the Mistermorph Console SPA for `mistermorph console serve`.

- Runtime: browser-side Vue3 + Vue Router
- Runtime deps: local `vue` + `vue-router`
- UI: local `quail-ui`
- Build: Vite (`src` -> `dist`)

## Build (production static)

1. Build frontend to `dist`:

```bash
cd web/console
pnpm install
pnpm build
```

2. Start daemon in one terminal (for current task list APIs):

```bash
MISTER_MORPH_SERVER_AUTH_TOKEN=dev-token \
go run ./cmd/mistermorph serve --server-auth-token dev-token
```

3. Start console in another terminal:

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

2. Start console backend (API origin for proxy):

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
- You only need `dist` for backend static serving; during frontend dev you mainly use Vite page.
