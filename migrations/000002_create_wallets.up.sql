-- Wallets table (balance only)
CREATE TABLE wallets (
    id CHAR(20) PRIMARY KEY,
    user_id CHAR(20) UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    balance_cents BIGINT NOT NULL DEFAULT 0 CHECK (balance_cents >= 0),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
