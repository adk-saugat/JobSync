# JobSync

CLI that checks Gmail for job application emails once a day, extracts details with **Gemini** (Google AI Studio free tier), and updates a **Google Sheet**.

## Status

**Phase 5** — full sync works end-to-end.

## Requirements

- Go 1.22+
- Google Cloud: Sheets + Gmail ([docs/GOOGLE_SETUP.md](docs/GOOGLE_SETUP.md))
- Gemini API key ([docs/GEMINI_SETUP.md](docs/GEMINI_SETUP.md))

## Build

```bash
make build
```

## Test

```bash
make test
```

## Commands

```bash
./bin/jobsync init
./bin/jobsync sync --dry-run --limit 5   # preview, no writes
./bin/jobsync sync --limit 10            # real sync → Sheet + SQLite
./bin/jobsync sync                       # default max 15 Gemini calls
./bin/jobsync status
```

## Layout

```text
cmd/jobsync/          # CLI entrypoint
internal/
  cli/                # init, sync, status commands
  config/             # ~/.config/jobsync settings
  domain/             # Application, SyncRun, statuses
  storage/            # SQLite + migrations
  google/
    auth/             # Google OAuth
    gmail/            # Gmail search/read
    sheets/           # tracker spreadsheet
  gemini/             # Gemini extraction (AI Studio)
  syncer/             # full sync orchestration (Phase 5)
  schedule/           # daily cron (Phase 6)
docs/
```

## Roadmap

See [docs/ROADMAP.md](docs/ROADMAP.md).

## Planned user flow

1. `jobsync init` — Google login, Gemini API key, auto daily schedule (random evening time)
2. Each day — cron runs `jobsync sync`
3. Open Google Sheet to see updates
