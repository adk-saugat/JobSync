CREATE TABLE IF NOT EXISTS applications (
    id TEXT PRIMARY KEY NOT NULL,
    company TEXT NOT NULL,
    position TEXT NOT NULL,
    status TEXT NOT NULL,
    applied_at TEXT,
    interview_at TEXT,
    oa_at TEXT,
    source_email_id TEXT UNIQUE,
    sheet_row_id TEXT,
    raw_excerpt TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_applications_company_position
    ON applications (company COLLATE NOCASE, position COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS email_processed (
    gmail_message_id TEXT PRIMARY KEY NOT NULL,
    application_id TEXT REFERENCES applications(id),
    processed_at TEXT NOT NULL,
    classification TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sync_runs (
    id TEXT PRIMARY KEY NOT NULL,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    status TEXT NOT NULL,
    emails_seen INTEGER NOT NULL DEFAULT 0,
    emails_updated INTEGER NOT NULL DEFAULT 0,
    errors INTEGER NOT NULL DEFAULT 0,
    gemini_calls INTEGER NOT NULL DEFAULT 0,
    gemini_skipped_prefilter INTEGER NOT NULL DEFAULT 0,
    watermark TEXT NOT NULL DEFAULT '',
    error_summary TEXT NOT NULL DEFAULT ''
);
