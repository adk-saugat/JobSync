# Gemini setup (Phase 4)

JobSync uses **Gemini** via a free [Google AI Studio](https://aistudio.google.com/) API key (not Vertex).

## 1. Create an API key

1. Open https://aistudio.google.com/apikey  
2. Create an API key  
3. Copy it  

## 2. Save it in JobSync

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

## 3. Smoke test (uses free quota — keep limit small)

```bash
./bin/jobsync sync --extract --limit 2
```

This:

- finds a few job-like Gmail messages  
- calls Gemini Flash  
- prints JSON (company / position / status / …)  
- does **not** write to Google Sheets yet  

## Free-tier tips

- Prefer `gemini-3.6-flash` (default)  
- Start with `--limit 2`  
- If you hit quota, wait until the next day  
- Re-running full sync later skips already-processed mail (Phase 5)

## Troubleshooting

| Problem | Fix |
|---------|-----|
| no Gemini API key | Run `jobsync init` or edit `config.json` |
| HTTP 429 / quota exceeded | Stop; try again tomorrow |
| Empty / bad JSON | Try another email; model output varies |
| Model not found / 404 | Use `gemini-3.6-flash` (or whatever Flash model AI Studio shows now) |
