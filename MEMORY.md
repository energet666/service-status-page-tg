# MEMORY.md

## Memory Maintenance

- Future Codex sessions should read this file after `AGENTS.md`.
- Update this file when project behavior, configuration, deployment assumptions, or important implementation decisions change.
- Keep entries brief and factual.

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

- The status page uses server-side web address availability checks as the primary visible health signal.
- `GET /api/status` returns current check results and the active admin announcement; `GET /api/checks` remains available for check-only data.
- `GET /api/status` also returns an optional `pinnedInfo` block for persistent instructions or other stable page text.
- Availability targets are read from `CHECKS_FILE`, defaulting to `checks.json`.
- When Telegram is enabled, background availability monitoring runs every `CHECKS_INTERVAL`, defaulting to `5m`.

## Decisions

- Use Telegram long polling, not webhooks.
- Use `ADMIN_IDS` from `.env` for Telegram admin authorization.
- Use JSON storage instead of a database.
- Serve built frontend from `web/dist` through the Go server.
- Docker image builds the Svelte frontend, embeds `web/dist`, runs the Go server as a non-root user, and uses `/app/data/state.json` plus `/app/checks.json` by default.
- `docker-compose.example.yml` is a deploy template for the GHCR image with `./data:/app/data` persistence, no `env_file`, and all runtime env vars listed with safe defaults.
- Keep bug reports unauthenticated, with in-memory IP rate limiting.
- Keep availability check results live-only; do not persist them in `data/state.json`.
- Use HTTP(S) GET from the Go server for availability checks, not browser-side checks or ICMP ping.
- Store the availability panel open/closed state in browser `localStorage`.
- Admin announcements are separate from service health. Use `/announce`, `/maintenance`, and `/incident` for active announcements, and `/clear` to remove the active announcement while recording the clear action in the chat.
- A separate persistent info block can be managed via Telegram with `/info` and `/clearinfo`; it is shown at the top of the page and is not part of the announcement feed.
- Availability check errors should be human-readable in the UI; do not expose raw transport error strings for common timeout, DNS, or connection failures.
- Background availability alerts notify Telegram admins when a problem appears or changes, and notify once when all targets recover. Manual UI checks do not affect monitor state.

## Recent Fixes

- The bug report form initially rendered labels and fields in a broken horizontal layout because daisyUI 5 did not apply the expected `form-control` behavior. It was fixed in `web/src/App.svelte` by using explicit `flex flex-col gap-*` layout classes and `w-full` on fields.
- The availability checks block was added to `web/src/App.svelte` and then made collapsible with persisted browser state.
- The availability block header now shows the overall check state and remains readable on narrow screens.
- Background availability monitoring and Telegram alerts were added, including recovery notifications after outages.

## Local Runtime Notes

- It is OK to start local servers when needed for verification, but stop them immediately after the check is complete.
- Do not leave long-running dev or backend servers running unless the user explicitly asks for a persistent server.
- If restarting the server fails with `address already in use`, find and stop the old process listening on the relevant port.
- `BOT_TOKEN` was not configured during local smoke tests, so Telegram integration was not exercised end to end.
