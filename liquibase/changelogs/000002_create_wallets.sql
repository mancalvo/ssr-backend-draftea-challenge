--liquibase formatted sql

--changeset mancalvo:000002-create-wallets
--comment: Wallets table (balance only)
CREATE TABLE wallets (
    id CHAR(20) PRIMARY KEY,
    user_id CHAR(20) UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    balance_cents BIGINT NOT NULL DEFAULT 0 CHECK (balance_cents >= 0),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

--rollback DROP TABLE wallets;
