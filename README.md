# JobSync

Track your job applications automatically.

JobSync reads your job emails from Gmail, figures out the status (applied, interview, rejected, etc.), and keeps a Google Sheet up to date. After setup, it syncs every day in the cloud — even when your laptop is closed.

**You need:** a Gmail account and a free [Gemini API key](https://aistudio.google.com/apikey).

---

## Choose how to set up

### Option A — Web (easiest)

1. Open **[the setup site](https://jobsync-b7ltqpwroa-uc.a.run.app)**
2. Sign in with Google
3. Paste your Gemini API key

Done. JobSync creates your tracker sheet and turns on daily cloud sync.

### Option B — Command line

#### 1. Download JobSync

Go to **[Releases](https://github.com/adk-saugat/JobSync/releases/latest)** and download the file for your computer:

- **Mac (M1/M2/M3):** `jobsync-darwin-arm64`
- **Mac (Intel):** `jobsync-darwin-amd64`
- **Linux:** `jobsync-linux-amd64`

```bash
cd ~/Downloads
chmod +x jobsync-darwin-arm64
```

*(Swap the filename if you downloaded a different one.)*

If Mac says the file “can’t be opened”: **System Settings → Privacy & Security → Open Anyway**.

#### 2. Run setup

```bash
./jobsync-darwin-arm64 init
```

This signs you into Google, creates a **JobSync Tracker** spreadsheet, and asks for your Gemini API key.

#### 3. Turn on daily sync

```bash
./jobsync-darwin-arm64 cloud push
```

Done. Your sheet updates automatically each day.

---

## Optional (CLI)

### Use `jobsync` from anywhere

```bash
mkdir -p ~/bin
mv ~/Downloads/jobsync-darwin-arm64 ~/bin/jobsync
echo 'export PATH=$HOME/bin:$PATH' >> ~/.zshrc
source ~/.zshrc
```

(Linux: use `~/.bashrc` instead of `~/.zshrc`.)

Then run `jobsync status` or `jobsync sync` from any folder.

### Manual sync anytime

```bash
jobsync sync
```

Rescan from a date (retries skipped/failed emails):

```bash
jobsync sync --since 2026-08-01
```

---

## Help

| If this happens… | Do this |
|------------------|---------|
| Google says “Access blocked” | Ask the person who shared JobSync to add your Gmail as a test user |
| It asks for a Gemini key | Get a key from [AI Studio](https://aistudio.google.com/apikey) and finish setup |
| Sign-in stopped working (CLI) | Delete `~/.config/jobsync/token.json`, then run `init` and `cloud push` again |
| Web setup fails after Google | Ask the maintainer to check the OAuth redirect URI (see [DEPLOY.md](docs/DEPLOY.md)) |

Web and CLI both register you for the **same** daily cloud sync. If you already set up on the web, `jobsync init` automatically reuses that Gmail’s existing tracker sheet — no manual config.

---

[Privacy policy](docs/PRIVACY.md) · [Maintainer docs](docs/DEPLOY.md)
