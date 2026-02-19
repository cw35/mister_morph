# Admin SPA (Minimal)

This directory contains a minimal static SPA for `mistermorph admin serve`.

- Runtime: browser-side Vue3 + Vue Router + Pinia (CDN modules)
- UI: tries to load `quail-ui` plugin from CDN
- Build step: not required for this minimal version

## Run

1. Start daemon in one terminal (for current task list APIs):

```bash
MISTER_MORPH_SERVER_AUTH_TOKEN=dev-token \
go run ./cmd/mistermorph serve --server-auth-token dev-token
```

2. Start admin in another terminal:

```bash
MISTER_MORPH_ADMIN_PASSWORD=secret \
MISTER_MORPH_SERVER_AUTH_TOKEN=dev-token \
go run ./cmd/mistermorph admin serve --admin-static-dir ./web/admin/dist
```

3. Open:

`http://127.0.0.1:9080/admin`
