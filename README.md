# JobSync

CLI that checks Gmail for job application emails once a day, extracts details with **Gemini** (Google AI Studio free tier), and updates a **Google Sheet**.

## Status

**Phase 4** — Gemini extraction. Use `jobsync sync --extract --limit 2`.

## Requirements

- Go 1.22+
- Google Cloud: Sheets + Gmail APIs ([docs/GOOGLE_SETUP.md](docs/GOOGLE_SETUP.md))
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
./bin/jobsync init                      # Google login + Gemini key
./bin/jobsync sync --emails-only        # list emails (no Gemini)
./bin/jobsync sync --extract --limit 2  # Gemini extract (no Sheet writes)
./bin/jobsync status
```

Local data: `~/.config/jobsync/` (override with `JOBSYNC_CONFIG_DIR`).

## Roadmap

See [docs/ROADMAP.md](docs/ROADMAP.md).

## Planned user flow

1. `jobsync init` — Google login, Gemini API key, auto daily schedule (random evening time)
2. Each day — cron runs `jobsync sync`
3. Open Google Sheet to see updates
