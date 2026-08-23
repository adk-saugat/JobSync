CREATE TABLE IF NOT EXISTS accounts (
    id TEXT PRIMARY KEY,
    spreadsheet_id TEXT NOT NULL DEFAULT '',
    sheet_name TEXT NOT NULL DEFAULT 'Applications',
    gemini_api_key TEXT NOT NULL DEFAULT '',
    gemini_model TEXT NOT NULL DEFAULT 'gemini-3.6-flash',
    oauth_token_json TEXT NOT NULL DEFAULT '',
    auth_scopes_version INTEGER NOT NULL DEFAULT 0,
    sync_hour INTEGER,
    sync_minute INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS applications (
    id TEXT PRIMARY KEY NOT NULL,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    company TEXT NOT NULL,
    position TEXT NOT NULL,
    status TEXT NOT NULL,
    applied_at TIMESTAMPTZ,
    interview_at TIMESTAMPTZ,
    oa_at TIMESTAMPTZ,
    source_email_id TEXT,
    sheet_row_id TEXT,
    raw_excerpt TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_applications_account_source_email
    ON applications (account_id, source_email_id)
    WHERE source_email_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_applications_account_company_position
    ON applications (account_id, lower(company), lower(position));

CREATE TABLE IF NOT EXISTS email_processed (
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    gmail_message_id TEXT NOT NULL,
    application_id TEXT REFERENCES applications(id),
    processed_at TIMESTAMPTZ NOT NULL,
    classification TEXT NOT NULL,
    PRIMARY KEY (account_id, gmail_message_id)
);

CREATE TABLE IF NOT EXISTS sync_runs (
    id TEXT PRIMARY KEY NOT NULL,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    status TEXT NOT NULL,
    emails_seen INTEGER NOT NULL DEFAULT 0,
    emails_updated INTEGER NOT NULL DEFAULT 0,
    errors INTEGER NOT NULL DEFAULT 0,
    gemini_calls INTEGER NOT NULL DEFAULT 0,
    gemini_skipped_prefilter INTEGER NOT NULL DEFAULT 0,
    watermark TEXT NOT NULL DEFAULT '',
    error_summary TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_sync_runs_account_started
    ON sync_runs (account_id, started_at DESC);
