# JobSync Roadmap (V1)

## What you're building

A **CLI** anyone can install that:

1. Connects their Gmail + Google Sheet (OAuth once)  
2. Once a day (end of day), checks **new** job emails  
3. Uses **Gemini** (AI Studio free tier) on as many as the daily quota allows  
4. Updates their **Google Sheet**  
5. Remembers progress in local **SQLite** so leftovers continue tomorrow  

**One-line V1 goal:**  
> Every evening, new job emails show up in my Sheet — without blowing Gemini’s free limit.

---

## Stack (locked)

| Piece | Choice |
|-------|--------|
| App | Go CLI (`jobsync`) |
| Database | SQLite (local file per user) |
| Email | Gmail API + OAuth |
| AI | Gemini Flash via AI Studio (free, limited daily quota) |
| Tracker | Google Sheets API + OAuth |
| Schedule | Once daily at a **random evening time** (auto-installed in `init`; user does not choose) |

**Not in V1:** Notion, Neon/Postgres required, Redis, RabbitMQ, React, always-on server.

---

## User workflow

### First time
1. Install CLI  
2. `jobsync init` → Google sign-in (Gmail + Sheets) → paste Gemini API key  
3. Init **automatically** installs a once-daily job at a **random evening time** (user does not pick a schedule)  

### Every day (automatic)
1. Cron runs `jobsync sync` once at that random time  
2. App finds emails **since last successful sync**  
3. Processes what Gemini free tier allows today  
4. Updates the Sheet  
5. If more emails remain or quota ran out → unfinished ones wait for **tomorrow**  

### Optional
- `jobsync sync` anytime (manual)  
- `jobsync status` → last run, next run time, emails left, quota stopped early?  

---

## How one daily sync works

```
Gmail (new since last sync)
    → pre-filter / skip already processed
    → Gemini (until daily free limit or inbox batch done)
    → SQLite + Google Sheet
```

**Quota-aware rules:**
1. Only **new** emails since last watermark  
2. Skip already-processed IDs (no Gemini)  
3. Cheap local pre-filter before Gemini  
4. Truncate bodies; use Flash model  
5. Cap Gemini calls per day (config + stop on 429/quota)  
6. Stop cleanly when quota is hit — **don’t fail the whole day**; save progress  
7. Next day’s cron continues where it left off  

---

## Build order

### Phase 0 — CLI that builds
- Go module, `cmd/jobsync`, folders, README  
- Commands stub: `init`, `sync`, `status`  
- Confirm: `go build` works  

### Phase 1 — SQLite + models
- Local DB file (e.g. `~/.config/jobsync/jobsync.db`)  
- Tables: applications, email_processed, sync_runs  
- Confirm: can insert/read rows  
- **Done** — see `internal/db` + `make test`  

### Phase 2 — Google Sheets
- OAuth; create or link tracker Sheet with headers  
- Append + update row by stable Row ID  
- Confirm: CLI can write a test row  
- **Done** — `jobsync init` + [docs/GOOGLE_SETUP.md](GOOGLE_SETUP.md)  

### Phase 3 — Gmail
- OAuth; search/read new mail since watermark  
- Confirm: list new emails with **no** Gemini calls  
- **Done** — `jobsync sync --emails-only` (+ enable Gmail API)  

### Phase 4 — Gemini
- User’s AI Studio key from `init`  
- Extract JSON; handle free-tier quota errors  
- Confirm: 1–2 emails extract correctly  
- **Done** — `jobsync sync --extract --limit 2`  

### Phase 5 — Daily sync (core V1)
- Wire full `jobsync sync`  
- Watermark + “continue tomorrow” if quota/emails remain  
- Confirm: one run updates Sheet; second run doesn’t duplicate  

### Phase 6 — Auto schedule + polish
- During `init`, pick a **random time once** in an evening window (e.g. 8:00–11:00 PM local), save it in config, install OS cron/launchd  
- User never sets or changes the schedule in V1  
- `status` shows the assigned time  
- Logging, safe first-run defaults  
- Confirm: daily job runs without the user watching  

---

## Commands (V1)

| Command | Purpose |
|---------|---------|
| `jobsync init` | Google login + Gemini key + Sheet setup + auto daily schedule |
| `jobsync sync` | One sync now (same logic cron uses) |
| `jobsync status` | Last sync, next run time, quota stop, pending mail |

---

## Done when (V1 checklist)

- [ ] `init` installs daily sync automatically (no schedule command)  
- [ ] Only **new** emails since last sync are considered  
- [ ] Gemini free tier is respected; overflow waits until next day  
- [ ] Sheet updates without duplicates  
- [ ] `status` shows last run + assigned next run time  

---

## Decisions already made

- CLI for other people (not a hosted SaaS)  
- SQLite (not Neon/local Postgres by default)  
- Gmail + Sheets via **OAuth**  
- Gemini via user’s AI Studio key  
- Sync = **once daily**, quota-aware  
- Schedule time = **random evening slot**, chosen automatically (user does not set it)  

## Decide when you get there

- Evening random window (e.g. 20:00–23:00 local)  
- Exact Gemini Flash model id  
- Max Gemini calls per day (start conservative, e.g. 20–50)  
- Cron vs launchd details on macOS/Linux (Windows later)  

---

## Tip

Get `jobsync sync` solid **before** wiring cron.  
Cron is just “run the same command once a day.”
