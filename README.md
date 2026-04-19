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

## Availability checks

The web page can ask the Go server to check configured web addresses from the server side.
Targets are read from `CHECKS_FILE`, defaulting to `checks.json`.
If the file is missing or has no targets, the backend falls back to YouTube and Instagram.

```json
{
  "targets": [
    { "name": "YouTube", "url": "https://www.youtube.com/" },
    { "name": "Instagram", "url": "https://www.instagram.com/" },
    { "name": "Telegram", "url": "https://web.telegram.org/" }
  ]
}
```

Bare domains like `youtube.com` are normalized to HTTPS.
The availability block in the UI is collapsible and stores its open/closed state in the browser.

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
