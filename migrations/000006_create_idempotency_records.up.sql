CREATE TABLE idempotency_records (
    key           VARCHAR(64) PRIMARY KEY,
    request_hash  VARCHAR(64) NOT NULL,
    status_code   INTEGER NOT NULL DEFAULT 0,
    response_body BYTEA,
    content_type  VARCHAR(128) DEFAULT 'application/json',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ NOT NULL
);

-- Index for cleanup job (delete expired records)
CREATE INDEX idx_idempotency_expires_at ON idempotency_records (expires_at);
