# JobSync

Gmail → Gemini → Google Sheets job application tracker.

Reads job-application emails, extracts status with Gemini, and updates a Google Sheet. Daily sync runs in the cloud after `cloud push`.

## Setup

```bash
jobsync init
jobsync cloud push
```

**Requirements:** Gmail account, free [Gemini API key](https://aistudio.google.com/apikey).

**Build from source:**

```bash
make build
./bin/jobsync init
./bin/jobsync cloud push
```

While the OAuth app is in testing mode, the maintainer must add your Gmail as a [test user](https://console.cloud.google.com/auth/audience?project=jobsync-506205).

## Commands

```bash
jobsync status
jobsync sync              # optional manual sync
jobsync sync --dry-run --limit 5
```

Config lives in `~/.config/jobsync/` (`token.json`, `config.json`, `jobsync.db`).

## Troubleshooting

| Problem | Fix |
|---------|-----|
| Access blocked on sign-in | Ask maintainer to add your Gmail as OAuth test user |
| No Gemini key | Run `jobsync init` |
| Need to re-login | Delete `token.json`, run `init`, then `cloud push` |

---

**Maintainer:** [docs/DEPLOY.md](docs/DEPLOY.md) · **Privacy:** [docs/PRIVACY.md](docs/PRIVACY.md)

```bash
make build && make test
make release VERSION=v0.1.0
```
