# JobSync

JobSync is a small CLI that keeps your job applications up to date.

It reads status emails from **Gmail** (applied, assessment, interview, rejected, accepted), extracts the details with **Gemini**, and writes them into a **Google Sheet** tracker. Progress is stored locally in SQLite so re-runs don’t duplicate rows and unfinished mail can continue on the next sync.

You do **not** need Google Cloud Console. You only need a free **Gemini API key** and a Google account to sign in.

---

## What you get

After setup you’ll have:

1. A **JobSync Tracker** Google Sheet with columns:

   | Company | Position | Status | Applied At | Interview At | Assessment At | Notes |

2. A local database at `~/.config/jobsync/jobsync.db`
3. A one-command sync: `jobsync sync`

Row colors in the Sheet: rejected (red), interview (blue), assessment (amber), accepted (green).

---

## Requirements

- **Go 1.22+** ([install Go](https://go.dev/dl/))
- A **Gmail** account (for sign-in + reading mail)
- A free **Gemini** API key from [Google AI Studio](https://aistudio.google.com/apikey)

---

## Install

```bash
git clone https://github.com/saugatadhikari/jobSync.git
cd jobSync
make build
```

The binary is `./bin/jobsync`. Optionally:

```bash
cp ./bin/jobsync /usr/local/bin/jobsync
```

---

## Gemini setup

JobSync uses **Gemini** via a free [Google AI Studio](https://aistudio.google.com/) API key (not Vertex).

### 1. Create an API key

1. Open https://aistudio.google.com/apikey  
2. Create an API key  
3. Copy it  

### 2. Save it in JobSync

```bash
make build
./bin/jobsync init
```

When prompted, paste the key. It is stored in `~/.config/jobsync/config.json` (not committed).

Or set it manually in that file:

```json
{
  "gemini_api_key": "YOUR_KEY",
  "gemini_model": "gemini-3.6-flash"
}
```

### 3. Smoke test (uses free quota — keep limit small)

```bash
./bin/jobsync sync --extract --limit 2
```

This finds a few job-like Gmail messages, calls Gemini Flash, prints JSON (company / position / status / …), and does **not** write to Google Sheets.

### Free-tier tips

- Prefer `gemini-3.6-flash` (default)
- Start with `--limit 2`
- If you hit quota, wait until the next day
- Re-running full sync later skips already-processed mail

---

## One-time setup (`init`)

```bash
./bin/jobsync init
```

This will:

1. Open a browser for Google sign-in (Gmail + Sheets access via JobSync’s built-in OAuth app)
2. Create (or refresh) your **JobSync Tracker** spreadsheet
3. Ask for your Gemini API key (if not already set)
4. Smoke-test Gmail search

Open the printed spreadsheet URL when it finishes.

> **Note:** While JobSync’s Google app is in testing mode, the maintainer may need to add your Gmail as a test user before sign-in works. If you see “access blocked,” ask them to add you.

---

## Everyday use

### Preview (no writes)

```bash
./bin/jobsync sync --dry-run --limit 5
```

### Sync for real

```bash
./bin/jobsync sync              # up to 15 Gemini calls (default)
./bin/jobsync sync --limit 10   # smaller batch
```

Each run:

1. Searches Gmail for application status emails
2. Skips mail already processed
3. Calls Gemini on new ones (until the limit or free-tier quota)
4. Upserts rows in SQLite + Google Sheets

If Gemini quota runs out, JobSync stops cleanly. Run again later — already-processed emails are skipped.

### Check setup

```bash
./bin/jobsync status
```

### Optional diagnostics

```bash
./bin/jobsync sync --emails-only --limit 10   # list matching mail (no Gemini)
./bin/jobsync sync --extract --limit 2        # Gemini extract only (no Sheet writes)
```

---

## Where files live

Under `~/.config/jobsync/`:

| File | Purpose |
|------|---------|
| `token.json` | Your Google login (created by `init`) |
| `config.json` | Spreadsheet ID, Gemini key / model |
| `jobsync.db` | Local SQLite (applications + processed emails) |

You should **not** need a `client_secret.json` — OAuth is built into the CLI.

---

## Tips

- Start with `--dry-run` or a small `--limit` while testing.
- Prefer the default Flash model (`gemini-3.6-flash`) on the free tier.
- Only emails that clearly include **company**, **position**, and a **status** are written to the Sheet.
- Re-run `jobsync init` anytime to refresh the Sheet layout or re-auth Google.

---

## Troubleshooting

| Problem | What to try |
|---------|-------------|
| Access blocked / app not verified | Ask the maintainer to add your Gmail as an OAuth **test user** |
| Browser doesn’t open | Paste the printed URL into your browser |
| No Gemini API key | Run `jobsync init` or set `gemini_api_key` in `config.json` |
| HTTP 429 / quota exceeded | Stop; try again tomorrow; lower `--limit` |
| Empty / bad Gemini JSON | Try another email; model output varies |
| Model not found / 404 | Use `gemini-3.6-flash` (or whatever Flash model AI Studio shows now) |
| Want a fresh Sheet | Remove `spreadsheet_id` from `config.json` and run `init` |
| Need to re-login | Delete `~/.config/jobsync/token.json` and run `init` |

Forks / self-hosted OAuth: set `JOBSYNC_CLIENT_SECRET_FILE` or place a Desktop OAuth JSON at `~/.config/jobsync/client_secret.json`.

---

## What’s next

**Daily auto-sync** (cron / launchd at a random evening time) is planned for Phase 6. Until then, run `jobsync sync` yourself when you want updates.

Roadmap: [ROADMAP.md](ROADMAP.md)

---

## Develop

```bash
make build   # ./bin/jobsync
make test
make tidy
```
