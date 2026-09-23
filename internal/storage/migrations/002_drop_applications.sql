CREATE TABLE email_processed_new (
    gmail_message_id TEXT PRIMARY KEY NOT NULL,
    processed_at TEXT NOT NULL,
    classification TEXT NOT NULL
);

INSERT INTO email_processed_new (gmail_message_id, processed_at, classification)
    SELECT gmail_message_id, processed_at, classification FROM email_processed;

DROP TABLE email_processed;
ALTER TABLE email_processed_new RENAME TO email_processed;

DROP TABLE IF EXISTS applications;
