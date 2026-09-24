# Strava Auto Kudos

Automatically give kudos to new activities from athletes you follow on Strava.

The public Strava API cannot list friends' activities or give kudos. This tool uses the same dashboard requests the Strava website uses, authenticated with your browser session cookie.

## How it works

Every poll (`POLL_INTERVAL`, default 2 minutes):

1. Loads your following feed: `GET /dashboard/feed?feed_type=following`
2. Pages with `before`/`cursor` until it has `STRAVA_FEED_LIMIT` latest items (default 20, max 100)
3. Gives kudos (`POST /feed/activity/{id}/kudo`) to anything with `canKudo: true`
4. Skips your own activities and anything already kudoed (`canKudo` flips to `false` after)

The session cookie is refreshed proactively via `POST /api/next/refresh-cookies`: on startup, 1 hour before expiry, and at least every 4 hours. Rotated cookies are saved to `data/session.json` (or `/data/session.json` in Docker).

## Setup

1. Log in to [strava.com](https://www.strava.com) in your browser.
2. Open DevTools → **Application** → **Cookies** → `https://www.strava.com`.
3. Copy the value of `_strava4_session`.
4. Create your `.env`:

```bash
cp .env.example .env
```

```
STRAVA_SESSION_COOKIE=paste-the-cookie-here
STRAVA_FEED_LIMIT=20
POLL_INTERVAL=2m
DATA_DIR=/data   # ./data for local runs
```

Your athlete id is resolved automatically from `GET /frontend/athletes/current`.

## Run with Docker Compose (recommended)

```bash
docker compose up -d --build
docker compose logs -f
```

Images are also published to GHCR on every push to `main` (`linux/amd64` + `linux/arm64`):

```bash
docker pull ghcr.io/ilyasaftr/strava-auto-kudos:latest
```

## Run locally

Requires Go 1.27+.

```bash
go run ./cmd/kudos run
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `STRAVA_SESSION_COOKIE` | — (required) | `_strava4_session` from browser cookies |
| `STRAVA_FEED_LIMIT` | `20` | Latest feed items per poll (1–100) |
| `POLL_INTERVAL` | `2m` | Time between polls |
| `DATA_DIR` | `./data` | Where `session.json` is stored |

## Development

```bash
go test ./...
go vet ./...
```
