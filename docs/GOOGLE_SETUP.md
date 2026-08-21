# Google setup

JobSync uses **Google Sheets** and **Gmail** via OAuth. You create a free Google Cloud OAuth client once, then `jobsync init` opens a browser to sign in.

## 1. Create a Google Cloud project

1. Open [Google Cloud Console](https://console.cloud.google.com/)
2. Create a project (or pick an existing one)

## 2. Enable APIs

**APIs & Services → Library**, enable both:

1. **Google Sheets API**
2. **Gmail API**

## 3. Configure the OAuth consent screen

1. **APIs & Services → OAuth consent screen**
2. User type: **External** (unless you use Google Workspace internal)
3. App name: `JobSync` (or anything)
4. Add your email as developer contact
5. Save
6. Under **Test users**, add **your Gmail address** (required while the app is in Testing)
7. Under **Data access / Scopes**, you can leave defaults — the CLI requests Sheets + Gmail readonly at login

## 4. Create a Desktop OAuth client

1. **APIs & Services → Credentials → Create credentials → OAuth client ID**
2. Application type: **Desktop app**
3. Name: `JobSync CLI`
4. Create → **Download JSON**

## 5. Install the client secret

```bash
mkdir -p ~/.config/jobsync
cp ~/Downloads/client_secret_*.json ~/.config/jobsync/client_secret.json
```

Never commit this file.

## 6. Run init

```bash
make build
./bin/jobsync init
```

This will:

1. Open a browser for Google sign-in (**Sheets + Gmail**)  
2. Create or refresh the **JobSync Tracker** spreadsheet  
3. Smoke-test Gmail search  

If you already signed in during Phase 2 (Sheets only), init will ask you to **sign in again** once to add Gmail permission.

### Sheet columns (what you see)

| Company | Position | Status | Applied At | Interview At | OA At | Notes |

Row ID is stored in a **hidden** first column so the app can update the correct row. Status values (color-coded): `applied`, `oa`, `interview`, `rejected`, `offer`, `other`.

## 7. List emails (Phase 3 check)

```bash
./bin/jobsync sync --emails-only
./bin/jobsync sync --emails-only --limit 5
```

This searches Gmail for job-like mail and prints them. **No Gemini calls.**

## Troubleshooting

| Problem | Fix |
|---------|-----|
| `missing Google OAuth client secret` | Copy the JSON to `~/.config/jobsync/client_secret.json` |
| Access blocked / app not verified | Add yourself as a **Test user**; use Advanced → continue |
| Gmail API error / 403 | Enable **Gmail API** for the project; re-run `jobsync init` to re-auth |
| Browser does not open | Copy the printed URL into your browser manually |
| Want a fresh sheet | Delete `spreadsheet_id` from `~/.config/jobsync/config.json` and run `init` again |
| Sheet still has old columns | Run `./bin/jobsync init` again — it clears the old layout and restyles |
| Need to re-login | Delete `~/.config/jobsync/token.json` and run `init` |
