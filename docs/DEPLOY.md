# Deploying JobSync

Maintainer guide. GCP project: **`jobsync-506205`** · Cloud Run: **`https://jobsync-b7ltqpwroa-uc.a.run.app`**

---

## OAuth consent screen

[Branding](https://console.cloud.google.com/auth/branding?project=jobsync-506205) · [Audience / test users](https://console.cloud.google.com/auth/audience?project=jobsync-506205) · [Scopes](https://console.cloud.google.com/auth/scopes?project=jobsync-506205)

| Field | Value |
|-------|--------|
| App name | `JobSync` |
| Support email | your Gmail |
| Home page | `https://github.com/adk-saugat/JobSync` |
| Privacy policy | `https://github.com/adk-saugat/JobSync/blob/main/docs/PRIVACY.md` |
| Authorized domains | `github.com` |

Scopes: `gmail.readonly`, `spreadsheets`. Enable **Gmail API** and **Sheets API**.

In **Testing** mode, add each user’s Gmail under **Test users** (max 100). Policy text: [PRIVACY.md](PRIVACY.md).

---

## OAuth credentials (never commit)

Keep Desktop OAuth JSON **local only**:

```bash
cp internal/google/auth/oauth_client.json.example internal/google/auth/oauth_client.json
# paste real client_id / client_secret from Google Cloud Console
```

| Where | How credentials are used |
|-------|---------------------------|
| **CLI release binaries** | `make release` bakes them in via `-ldflags` from `oauth_client.json` |
| **Cloud Run** | Set `GOOGLE_OAUTH_CLIENT_ID` + `GOOGLE_OAUTH_CLIENT_SECRET` in `deploy.env.yaml` |
| **GitHub** | Never — file is gitignored |

Users of release binaries need no OAuth file. Forks without baked credentials set `JOBSYNC_CLIENT_SECRET_FILE` or put JSON at `~/.config/jobsync/client_secret.json`.

---

## Neon + Cloud Run

`deploy.env.yaml` (from `deploy.env.yaml.example`):

```yaml
DATABASE_URL: "postgresql://...?sslmode=require"
SYNC_SECRET: "openssl-rand-hex-32"
GOOGLE_OAUTH_CLIENT_ID: "....apps.googleusercontent.com"
GOOGLE_OAUTH_CLIENT_SECRET: "GOCSPX-..."
```

```bash
gcloud config set project jobsync-506205

gcloud run deploy jobsync \
  --source . \
  --region us-central1 \
  --project jobsync-506205 \
  --env-vars-file deploy.env.yaml \
  --allow-unauthenticated \
  --memory 512Mi \
  --timeout 540
```

Also set:

```yaml
PUBLIC_BASE_URL: "https://jobsync-b7ltqpwroa-uc.a.run.app"
```

### Web setup OAuth (React wizard)

The setup UI is served from the same Cloud Run URL (`/`).

In Google Cloud Console → Credentials → OAuth client:

1. Use a **Web application** client (recommended), or add redirect URIs if your client allows it.
2. Authorized redirect URI:

`https://jobsync-b7ltqpwroa-uc.a.run.app/setup/oauth/callback`

3. Local testing: `http://127.0.0.1:8080/setup/oauth/callback`

Flow: `/setup/oauth/start` → Google → paste Gemini key → creates Sheet + registers daily sync.

---

## Cloud Scheduler

Daily sync for all users:

```bash
gcloud scheduler jobs create http jobsync-daily \
  --location us-central1 \
  --project jobsync-506205 \
  --schedule "30 21 * * *" \
  --time-zone "America/Chicago" \
  --uri "https://jobsync-b7ltqpwroa-uc.a.run.app/sync/all" \
  --http-method POST \
  --headers "Authorization=Bearer YOUR_SYNC_SECRET"
```

Test: `curl -X POST "https://jobsync-b7ltqpwroa-uc.a.run.app/sync/all" -H "Authorization: Bearer YOUR_SYNC_SECRET" --max-time 600`

---

## Ship to users

```bash
make release VERSION=v0.1.0
```

Attach `dist/jobsync-*` to a GitHub Release. Users run `init` + `cloud push` only.

Update `CLOUD_URL` in the Makefile if the service URL changes, then rebuild.

---

## Redeploy

| Changed | Action |
|---------|--------|
| `internal/cloud/`, `cmd/server/` | `gcloud run deploy ...` |
| `internal/cli/`, `cmd/jobsync/` | `make release` |
| Secrets only | `gcloud run services update jobsync --env-vars-file deploy.env.yaml` |

Never commit `deploy.env.yaml`.
