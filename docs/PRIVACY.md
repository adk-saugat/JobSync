# JobSync Privacy Policy

**Last updated:** September 2, 2026  
**Contact:** adhikarisaugat34@gmail.com

JobSync helps you track job applications by reading relevant Gmail messages and updating a Google Sheet you own. You can set it up in the browser or with the command-line tool.

---

## What data JobSync accesses

| Data | Why |
|------|-----|
| **Gmail (read-only)** | Find emails about applications, interviews, assessments, rejections, and offers |
| **Google Sheets** | Create and update your personal “JobSync Tracker” spreadsheet |
| **Google account email** | Identify your account for cloud sync registration |

JobSync does **not** send email, delete mail, or access Gmail labels beyond what is needed to search and read message content for extraction.

---

## What JobSync stores

### On your computer (`~/.config/jobsync/`)

- Google OAuth token (so you stay signed in)
- Gemini API key (you provide this)
- Spreadsheet ID and app settings
- Local SQLite database (processed message IDs and application rows for manual sync)

### If you use cloud sync (web setup or `jobsync cloud push`)

With your permission, the same credentials and settings above are stored in the maintainer’s **Neon Postgres** database so daily sync can run on a server. Each user’s data is isolated by account id (derived from your Gmail address).

Gemini API keys and Google OAuth tokens are **encrypted at rest** in the database (AES-GCM) using a server-side key. They are decrypted only in memory when a sync runs.

Web setup stores a short-lived encrypted session cookie only while you finish Google sign-in and paste your Gemini key; it is cleared when setup completes.

The maintainer does **not** sell or share your data with third parties for advertising.

---

## Third-party services

- **Google** — Gmail, Sheets, and OAuth sign-in ([Google Privacy Policy](https://policies.google.com/privacy))
- **Google Gemini (AI Studio)** — extracts company, role, and status from email text using **your** API key ([Google AI terms](https://ai.google.dev/gemini-api/terms))
- **Neon** — hosted Postgres for cloud sync only ([Neon privacy](https://neon.tech/privacy-policy))

Email content sent to Gemini is limited to what is needed for extraction. JobSync does not train models on your data.

---

## How long data is kept

- **Local data** — until you delete `~/.config/jobsync/`
- **Cloud data** — until you ask the maintainer to remove your account or you stop using cloud sync
- **OAuth tokens** — refreshed automatically; revoke anytime in [Google Account permissions](https://myaccount.google.com/permissions)

---

## Your choices

- **Stop using JobSync** — delete `~/.config/jobsync/` and revoke app access in your Google Account
- **Cloud sync only** — finish web setup or run `jobsync init` + `jobsync cloud push`; daily sync runs on the server
- **No cloud** — use `jobsync sync` manually; nothing is sent to Neon unless you complete cloud registration

---

## Security

- OAuth tokens and API keys are stored with restrictive file permissions locally
- Cloud Gemini keys and OAuth tokens are encrypted at rest (AES-GCM); decrypted only for sync
- Cloud database connections use TLS (`sslmode=require`)
- Sync endpoints require maintainer secrets; users register with their own Google login

---

## Changes

This policy may be updated as the app evolves. The “Last updated” date at the top will change when it does.

---

## Contact

Questions or deletion requests: **adhikarisaugat34@gmail.com**
