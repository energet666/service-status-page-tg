# AGENTS.md

## Project

This is a small service status page:

- Backend: Go, standard `net/http`, `github.com/joho/godotenv`, `gopkg.in/telebot.v4`.
- Storage: JSON file, default `data/state.json`, with atomic temp-file write + rename.
- Availability checks: JSON config file, default `checks.json`, configured by `CHECKS_FILE`.
- Frontend: Vite + Svelte 5 runes, Tailwind CSS 4, daisyUI 5, `lucide-svelte`.
- Production shape: one Go process serves `/api/*`, static files from `web/dist`, and the Telegram bot.

## Commands

Use local Go caches inside `/tmp` when running in restricted environments:

```sh
env GOCACHE=/tmp/service-status-page-go-build GOMODCACHE=/tmp/service-status-page-go-mod go test ./...
```

Frontend:

```sh
cd web
npm install
npm run build
```

Run:

```sh
go run ./cmd/server
```

If `BOT_TOKEN` is empty, the HTTP server still starts and Telegram integration is disabled.

## Conventions

- Keep backend code under `internal/*` and the executable under `cmd/server`.
- Keep the frontend as a plain Vite SPA, not SvelteKit.
- Do not introduce a database for the current scale; use the JSON store unless requirements change.
- API errors should remain JSON: `{ "error": "..." }`.
- Keep public API routes under `/api`.
- Keep availability checks live-only; do not store probe results in `data/state.json`.
- Build the frontend before expecting Go to serve updated UI assets.
- Do not commit `.env`, `data/`, `web/dist/`, or `web/node_modules/`.

## Working Memory

- In a new chat/session, read `AGENTS.md` and `MEMORY.md` before making project changes.
- Treat `MEMORY.md` as project memory for current decisions, implementation notes, constraints, and important prior context.
- Update `MEMORY.md` when project behavior, configuration, deployment assumptions, or important implementation decisions change.
- Keep `MEMORY.md` concise: use short factual bullets, not transcripts or temporary debugging notes.
- If `MEMORY.md` conflicts with source code or `AGENTS.md`, trust source code first, then `AGENTS.md`; note the conflict if it matters for the task.

## UX Notes

- The first screen is the working status page, not a landing page.
- The availability checks block is collapsible and its open/closed state is stored in browser memory.
- The bug report form should remain visually aligned as a vertical form.
- Prefer explicit Tailwind layout classes over relying on daisyUI helper classes when alignment matters.
