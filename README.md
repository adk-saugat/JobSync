# JobSync

CLI that checks Gmail for job application emails once a day, extracts details with **Gemini** (Google AI Studio free tier), and updates a **Google Sheet**.

## Status

**Phase 3** — Gmail search/read. Use `jobsync sync --emails-only` (no Gemini yet).

## Requirements

- Go 1.22+
- Google Cloud: Sheets API + Gmail API + Desktop OAuth client ([docs/GOOGLE_SETUP.md](docs/GOOGLE_SETUP.md))

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
./bin/jobsync init                 # Google login (Sheets + Gmail) + Sheet setup
./bin/jobsync sync --emails-only   # list job-like emails (no Gemini)
./bin/jobsync status
./bin/jobsync help
```

Local data: `~/.config/jobsync/` (override with `JOBSYNC_CONFIG_DIR`).

## Roadmap

See [docs/ROADMAP.md](docs/ROADMAP.md).

## Planned user flow

1. `jobsync init` — Google login, Gemini API key, auto daily schedule (random evening time)
2. Each day — cron runs `jobsync sync`
3. Open Google Sheet to see updates
