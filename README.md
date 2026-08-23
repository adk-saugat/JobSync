# JobSync

Track your job applications automatically.

JobSync reads your job emails from Gmail, figures out the status (applied, interview, rejected, etc.), and keeps a Google Sheet up to date for you. After setup, it syncs every day in the cloud — even when your Mac is closed.

**You need:** a Gmail account and a free [Gemini API key](https://aistudio.google.com/apikey).

---

## Get started (3 steps)

### 1. Download JobSync

Go to **[Releases](https://github.com/adk-saugat/JobSync/releases/latest)** and download the file for your computer:

- **Mac (M1/M2/M3):** `jobsync-darwin-arm64`
- **Mac (Intel):** `jobsync-darwin-amd64`
- **Linux:** `jobsync-linux-amd64`

Open Terminal, go to your Downloads folder, and make it runnable:

```bash
cd ~/Downloads
chmod +x jobsync-darwin-arm64
```

*(Swap the filename if you downloaded a different one.)*

If Mac says the file “can’t be opened”: **System Settings → Privacy & Security → Open Anyway**.

### 2. Run setup

```bash
./jobsync-darwin-arm64 init
```

This will:

- Sign you into Google
- Create a **JobSync Tracker** spreadsheet
- Ask for your Gemini API key (paste it from [AI Studio](https://aistudio.google.com/apikey))

### 3. Turn on daily sync

```bash
./jobsync-darwin-arm64 cloud push
```

Done. Your sheet will update automatically each day.

---

## Optional

Check everything looks good:

```bash
./jobsync-darwin-arm64 status
```

Sync manually anytime:

```bash
./jobsync-darwin-arm64 sync
```

---

## Help

| If this happens… | Do this |
|------------------|---------|
| Google says “Access blocked” | Ask the person who shared JobSync to add your Gmail as a test user |
| It asks for a Gemini key | Run `init` again or get a key from [AI Studio](https://aistudio.google.com/apikey) |
| Sign-in stopped working | Delete `~/.config/jobsync/token.json`, then run `init` and `cloud push` again |

---

[Privacy policy](docs/PRIVACY.md) · [Maintainer docs](docs/DEPLOY.md)
