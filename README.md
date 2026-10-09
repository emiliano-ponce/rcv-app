# RankedChoice (rcv-app)

A free, no-signup web app for running ranked choice (instant-runoff) polls: create a poll, share the link, and watch round-by-round results update live.

## Features

- **Create polls without an account.** Give the poll a title, an optional description, and 3–10 candidates. Each poll gets a random 8-character share key (`/polls/{key}`).
- **Drag-to-rank ballots.** Candidates appear in random order on the ballot, and voters drag them into their preferred order.
- **Ranked choice tabulation** with a round-by-round breakdown and a plain-language explanation of each elimination.
- **Deterministic tie-breaking.** When candidates tie for last place, the one with the lowest Borda count is eliminated. If their Borda counts are also tied, the candidate with the lowest ID is eliminated.
- **Live results.** The results page polls an HTMX fragment endpoint for updates.
- **Poll management** (`/polls/{key}/manage`): edit the title and description, add, rename, or remove candidates (at least 3 must remain), close the poll, or delete it.
- **Abuse protection:**
  - Cloudflare Turnstile check on vote submission (verified server-side against Cloudflare's `siteverify` API)
  - Per-IP rate limits on poll creation and voting (fixed one-minute window, kept in memory, so limits reset on restart and are not shared across instances). The client IP comes from `CF-Connecting-IP`, then `X-Forwarded-For`, then the connection address
  - One-vote-per-browser cookie
  - Honeypot field on the create form
  - Same-origin (Origin/Referer) checks on state-changing management endpoints
- **Automatic cleanup.** A background worker deletes polls older than 14 days. It runs at startup and then every 24 hours.

## Tech stack

- **Backend:** Go (`net/http` with method/path routing, `html/template`)
- **Database:** SQLite/libSQL through [`libsql-client-go`](https://github.com/tursodatabase/libsql-client-go) and [`modernc.org/sqlite`](https://modernc.org/sqlite). It works with a local file or a remote libSQL/Turso database.
- **Frontend:** server-rendered templates, [HTMX](https://htmx.org), and plain JS/CSS in `ui/static/`
- **Tooling:** [Air](https://github.com/air-verse/air) for live reload, `staticcheck` for linting, and Prettier with `prettier-plugin-go-template` for template formatting
- **Deployment:** multi-stage Dockerfile (distroless runtime image)

## Prerequisites

- Go **1.26.1** or newer (see `go.mod`)
- Optional: `air` (live reload), `staticcheck` (lint), Node/npm (Prettier only), Docker

## Getting started

```bash
git clone https://github.com/emiliano-ponce/rcv-app.git
cd rcv-app
cp .env.example .env   # optional; see Configuration
go run ./cmd/server/main.go
```

Then open http://localhost:8080.

> Run the server from the repository root. Templates (`ui/html`), static files (`ui/static`), and migrations (`db/migrations`) are loaded from relative paths at runtime.

On first start, the app creates the database (a local `dev.db` by default) and applies the SQL files in `db/migrations/`. Applied versions are tracked in a `schema_migrations` table.

### Make targets

```bash
make run     # go run ./cmd/server/main.go
make build   # go build -o rcv-app ./cmd/server/main.go
make dev     # go mod tidy, then live reload with air (-c .air.toml)
make test    # go test ./...
make fmt     # gofmt -s -w .
make lint    # staticcheck ./...
make check   # fmt + lint
make clean   # go clean and remove the rcv-app binary
```

> `.air.toml` builds to `tmp/main.exe` with a Windows-style binary path, and it proxies the app (port 8080) on port 8081.

### Docker

```bash
docker build -t rcv-app .
docker run -p 10000:10000 rcv-app
```

The runtime image sets `APP_ENV=production` and `PORT=10000`, and exposes port 10000. In production, also set `DATABASE_URL`, `DATABASE_AUTH_TOKEN` (for a remote libSQL database), and the Turnstile keys.

## Configuration

When `DATABASE_URL` is unset, environment variables are loaded from a `.env` file if one exists. See `.env.example` for the full list.

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` (`10000` in the Docker image) | HTTP listen port. |
| `DATABASE_URL` | `file:dev.db` | libSQL/SQLite connection URL. Use a local `file:` URL or a remote `libsql://`/`https://` URL. |
| `DATABASE_AUTH_TOKEN` | – | Auth token for remote libSQL. Required for remote URLs when `APP_ENV` is `production`/`prod`. |
| `APP_ENV` | – | `dev`/`development`/`local` turns on dev defaults (see below). `production`/`prod` enforces the DB auth token. |
| `CF_TURNSTILE_SITE_KEY` | – | Cloudflare Turnstile site key. `.env.example` uses Cloudflare's test keys. |
| `CF_TURNSTILE_SECRET_KEY` | – | Cloudflare Turnstile secret key. |
| `DISABLE_TURNSTILE_VERIFY` | `true` in dev if either key is missing, otherwise `false` | Skips Turnstile verification on votes. |
| `ALLOW_DEV_MULTI_VOTE` | `true` in dev, otherwise `false` | Lets one browser vote more than once. Turns off the vote cookie guard and the vote rate limit. |
| `RCV_CREATE_RATE_LIMIT_PER_MIN` | `5` | Poll creations allowed per IP per minute. |
| `RCV_VOTE_RATE_LIMIT_PER_MIN` | `15` | Vote submissions allowed per IP per minute. |
| `RCV_MIGRATIONS_DIR` | `db/migrations` | Overrides the location of the migrations directory. |

## Routes

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/`, `/about`, `/contact` | Landing, about, and contact pages |
| GET | `/robots.txt`, `/sitemap.xml` | SEO files |
| POST | `/find` | Look up a poll by key |
| POST | `/polls` | Create a poll (rate limited) |
| GET | `/polls/{key}` | Ballot page |
| POST | `/polls/{key}/vote` | Submit a ranked ballot |
| GET | `/polls/{key}/results` | Results page |
| GET | `/polls/{key}/results/fragment` | HTMX results fragment for live updates |
| GET | `/polls/{key}/manage` | Poll management page |
| PATCH / DELETE | `/polls/{key}` | Edit title/description, or delete the poll (same-origin only) |
| POST | `/polls/{key}/candidates` | Add a candidate (same-origin only) |
| PATCH / DELETE | `/polls/{key}/candidates/{id}` | Rename or remove a candidate (same-origin only) |
| POST | `/polls/{key}/close` | Close the poll to new votes (same-origin only) |

## How tabulation works

`voting.Tabulate` in `internal/voting` runs these steps in order:

1. Each ballot counts toward its highest-ranked candidate who is still active.
2. If a candidate has **strictly more than 50%** of the active votes, that candidate wins.
3. Otherwise, the candidate with the fewest votes is eliminated. Ties are broken in this order:
   1. **Borda count** over all ballots, counting active candidates only (1st = N−1 points, …, last = 0). The lowest score is eliminated.
   2. **Lowest candidate ID** if the Borda scores are also equal.
4. Steps 1–3 repeat until a candidate wins or every ballot is exhausted.

## Tests

```bash
go test ./...                    # all tests
go test -v ./...                 # verbose
go test ./internal/voting/...    # tabulation and tie-breaking only
```

What the tests cover:

- **`internal/voting`:** majority wins, vote transfers between rounds, round structure, empty input, Borda score computation, and both tie-break layers (Borda, then lowest ID).
- **`internal/handlers`:** the create-form honeypot, the poll-creation rate limit, missing Turnstile tokens, the one-vote-per-browser cookie and its dev multi-vote override, the redirect to results with the "ballot received" toast, closing a poll without the owner cookie, and the same-origin middleware.
- **`internal/security`:** the fixed-window rate limiter and its HTTP wrapper.
- **`internal/database`:** deleting a poll cascades to its candidates, ballots, and rankings.

GitHub Actions (`.github/workflows/build-and-test.yml`) runs `go build -v ./...` and `go test -v ./...` with the Go version from `go.mod` on every push and pull request to `main`.

## Project structure

```
.
├── cmd/server/main.go        # Entry point: config, DB init, routing, server start
├── internal/
│   ├── voting/               # RCV tabulation and tie-breaking (HTTP-agnostic)
│   ├── models/               # Shared data structs (Poll, Candidate, Ballot, RoundResult)
│   ├── database/             # DB connection, migration runner, expired-poll cleanup
│   ├── handlers/             # HTTP handlers, template cache, rendering helpers
│   └── security/             # Rate limiting, Turnstile verification, same-origin checks, client IP
├── ui/
│   ├── html/                 # base.html layout, pages/, partials/
│   └── static/               # Page CSS and JS
├── db/migrations/            # SQL migrations, applied automatically at startup
├── Dockerfile
├── Makefile
├── .air.toml                 # Live-reload config
├── .env.example              # Sample environment configuration
└── AGENTS.md                 # Architecture and conventions for AI coding agents
```
