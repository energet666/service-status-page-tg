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

## Docker image

The GitHub Actions release workflow publishes an image to GitHub Container Registry:

```text
ghcr.io/energet666/service-status-page-tg:deploy-latest
ghcr.io/energet666/service-status-page-tg:sha-<commit>
```

Run it locally with a mounted data directory:

```sh
docker run --rm -p 8080:8080 -v "$(pwd)/data:/app/data" ghcr.io/energet666/service-status-page-tg:deploy-latest
```

Or use the Compose template:

```sh
cp docker-compose.example.yml docker-compose.yml
docker compose up -d
```

The image includes the built frontend and the default `checks.json`.
The Compose template lists all runtime environment variables with safe defaults; fill `BOT_TOKEN` and `ADMIN_IDS` to enable Telegram.

## Availability checks

The availability block is the primary service health signal.
Its header shows an overall badge: green when every target is available, yellow when some targets are unavailable, and red when all targets are unavailable.
Targets are read from `CHECKS_FILE`, defaulting to `checks.json`.
If the file is missing or has no targets, the backend falls back to YouTube and Instagram.
When Telegram is enabled, the backend also runs these checks in the background.
`CHECKS_INTERVAL` controls the interval and defaults to `5m`.
If a target becomes unavailable or the failure details change, administrators from `ADMIN_IDS` receive a Telegram report.
Set `CHECKS_INTERVAL=0` to disable the background monitor without disabling the UI checks.

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
The latest active admin announcement is shown above the availability block.
Use `/clear` when the announcement is no longer relevant; the clear action is recorded in the status chat.

## Telegram commands

```text
/announce текст объявления
/maintenance [текст объявления]
/incident [текст объявления]
/clear [текст записи]
/delete_last
/list
/help
```

Announcements have three visible types: plain, maintenance, and incident.
They do not directly set the service health badge; availability checks do that.
Only Telegram user ids from `ADMIN_IDS` can use admin commands.
