-- Payment & Wallet System - Complete Schema
-- Database: PostgreSQL

-- Users table
CREATE TABLE users (
    id CHAR(20) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Wallets table (balance only)
CREATE TABLE wallets (
    id CHAR(20) PRIMARY KEY,
    user_id CHAR(20) UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    balance_cents BIGINT NOT NULL DEFAULT 0 CHECK (balance_cents >= 0),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Offerings catalog
CREATE TABLE offerings (
    id CHAR(20) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    duration_days INT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Transactions ledger (all payment types)
CREATE TABLE transactions (
    id CHAR(20) PRIMARY KEY,
    user_id CHAR(20) REFERENCES users(id),
    wallet_id CHAR(20) REFERENCES wallets(id),
    type VARCHAR(20) NOT NULL CHECK (type IN ('deposit', 'purchase', 'refund')),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending', 
        'provider_charged', 
        'completed', 
        'provider_failed', 
        'wallet_failed',
        'wallet_credited',
        'entitlement_revoked', 
        'credit_failed', 
        'revoke_failed'
    )),
    offering_id CHAR(20) REFERENCES offerings(id),
    provider_ref VARCHAR(255),
    idempotency_key VARCHAR(64),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- User access to offerings
CREATE TABLE entitlements (
    id CHAR(20) PRIMARY KEY,
    user_id CHAR(20) REFERENCES users(id),
    offering_id CHAR(20) REFERENCES offerings(id),
    transaction_id CHAR(20) REFERENCES transactions(id),
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    granted_at TIMESTAMPTZ DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

-- Indexes for performance
CREATE INDEX idx_transactions_user ON transactions(user_id, created_at DESC);
CREATE INDEX idx_transactions_wallet ON transactions(wallet_id);
CREATE INDEX idx_entitlements_user ON entitlements(user_id);
CREATE INDEX idx_entitlements_offering ON entitlements(offering_id);
CREATE INDEX idx_entitlements_transaction ON entitlements(transaction_id);

-- Unique constraint: one active entitlement per user+offering
CREATE UNIQUE INDEX idx_entitlements_user_offering_active 
ON entitlements (user_id, offering_id) 
WHERE status = 'active';

-- Unique idempotency key per user (prevents cross-user key leakage)
CREATE UNIQUE INDEX idx_transactions_user_idempotency_key 
ON transactions (user_id, idempotency_key) 
WHERE idempotency_key IS NOT NULL;

-- Seed data for testing (using valid xid format)
INSERT INTO users (id, email, name, is_active) VALUES
    ('d60g19m0u7j2796eeac0', 'test@example.com', 'Test User', true),
    ('d60g19m0u7j2796eeacg', 'inactive@example.com', 'Inactive User', false);

INSERT INTO wallets (id, user_id, balance_cents) VALUES
    ('d60g19m0u7j2796eead0', 'd60g19m0u7j2796eeac0', 100000),
    ('d60g19m0u7j2796eeadg', 'd60g19m0u7j2796eeacg', 0);

INSERT INTO offerings (id, name, description, price_cents, is_active) VALUES
    ('d60g19m0u7j2796eeae0', 'Premium Plan', 'Monthly premium subscription', 9900, true),
    ('d60g19m0u7j2796eeaeg', 'Basic Plan', 'Monthly basic subscription', 4900, true),
    ('d60g19m0u7j2796eeaf0', 'One-time Feature', 'Unlock special feature', 1999, true);
