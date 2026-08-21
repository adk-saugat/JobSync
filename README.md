# JobSync

CLI that checks Gmail for job application emails once a day, extracts details with **Gemini** (Google AI Studio free tier), and updates a **Google Sheet**.

## Status

**Phase 0** — project skeleton. Commands are stubs.

## Requirements

- Go 1.22+

## Build

```bash
make build
# or
go build -o bin/jobsync ./cmd/jobsync
```

## Commands (stubs for now)

```bash
./bin/jobsync init      # setup (later)
./bin/jobsync sync      # run sync (later)
./bin/jobsync status    # status (later)
./bin/jobsync help
```

## Roadmap

See [docs/ROADMAP.md](docs/ROADMAP.md).

## Planned user flow

1. `jobsync init` — Google login, Gemini API key, auto daily schedule (random evening time)
2. Each day — cron runs `jobsync sync`
3. Open Google Sheet to see updates
