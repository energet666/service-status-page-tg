# Service Status Page

Go + Telegram bot + Svelte 5 status page for small services.

## Run locally

```sh
cp .env.example .env
cd web
npm install
npm run build
cd ..
go run ./cmd/server
```

Open http://localhost:8080.

If `BOT_TOKEN` is empty, the HTTP server still runs and Telegram integration is disabled.

## Telegram commands

```text
/ok [текст статуса]
/maintenance [текст статуса]
/incident [текст статуса]
/announce текст объявления
/resolve текст восстановления
/delete_last
/list
/help
```

Only Telegram user ids from `ADMIN_IDS` can use admin commands.
