# Google setup (Phase 2)

JobSync talks to **Google Sheets** with OAuth. You create a free Google Cloud OAuth client once, then `jobsync init` opens a browser to sign in.

## 1. Create a Google Cloud project

1. Open [Google Cloud Console](https://console.cloud.google.com/)
2. Create a project (or pick an existing one)

## 2. Enable the Sheets API

1. Go to **APIs & Services → Library**
2. Search for **Google Sheets API** → **Enable**

(Gmail API is enabled later in Phase 3.)

## 3. Configure the OAuth consent screen

1. **APIs & Services → OAuth consent screen**
2. User type: **External** (unless you use Google Workspace internal)
3. App name: `JobSync` (or anything)
4. Add your email as developer contact
5. Save
6. Under **Test users**, add **your Gmail address** (required while the app is in Testing)



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

1. Open a browser for Google sign-in  
2. Create a **JobSync Tracker** spreadsheet (or refresh an existing one)  
3. Style the sheet (header, status colors, dropdown) and write a test row  

### Sheet columns (what you see)

| Company | Position | Status | Applied At | Interview At | OA At | Notes |

Row ID is stored in a **hidden** first column so the app can update the correct row. Status values (color-coded): `applied`, `oa`, `interview`, `rejected`, `offer`, `other`.


Then open the printed Sheet URL and confirm it looks right.

## Troubleshooting

| Problem | Fix |
|---------|-----|
| `missing Google OAuth client secret` | Copy the JSON to `~/.config/jobsync/client_secret.json` |
| Access blocked / app not verified | Add yourself as a **Test user** on the consent screen |
| Browser does not open | Copy the printed URL into your browser manually |
| Want a fresh sheet | Delete `spreadsheet_id` from `~/.config/jobsync/config.json` and run `init` again |
| Sheet still has old columns | Run `./bin/jobsync init` again — it clears the old layout and restyles |

