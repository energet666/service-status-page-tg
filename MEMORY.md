# MEMORY.md

## Current State

- Project was created from scratch in `/Users/sh/projects/service-status-page`.
- Git was initialized after the initial implementation.
- Go tests pass with:

```sh
env GOCACHE=/tmp/service-status-page-go-build GOMODCACHE=/tmp/service-status-page-go-mod go test ./...
```

- Frontend build passes with:

```sh
cd web
npm run build
```

## Decisions

- Use Telegram long polling, not webhooks.
- Use `ADMIN_IDS` from `.env` for Telegram admin authorization.
- Use JSON storage instead of a database.
- Serve built frontend from `web/dist` through the Go server.
- Keep bug reports unauthenticated, with in-memory IP rate limiting.

## Recent Fixes

- The bug report form initially rendered labels and fields in a broken horizontal layout because daisyUI 5 did not apply the expected `form-control` behavior. It was fixed in `web/src/App.svelte` by using explicit `flex flex-col gap-*` layout classes and `w-full` on fields.

## Local Runtime Notes

- A server may already be running on `http://localhost:8080` from manual smoke tests.
- If restarting the server fails with `address already in use`, find and stop the old process listening on port 8080.
- `BOT_TOKEN` was not configured during local smoke tests, so Telegram integration was not exercised end to end.

