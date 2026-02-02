-- Payment & Wallet System - Complete Schema
-- Database: PostgreSQL

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Wallets table (balance only)
CREATE TABLE wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    balance_cents BIGINT NOT NULL DEFAULT 0 CHECK (balance_cents >= 0),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Offerings catalog
CREATE TABLE offerings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    duration_days INT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Transactions ledger (all payment types)
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    wallet_id UUID REFERENCES wallets(id),
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
    offering_id UUID REFERENCES offerings(id),
    provider_ref VARCHAR(255),
    idempotency_key VARCHAR(64),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- User access to offerings
CREATE TABLE entitlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    offering_id UUID REFERENCES offerings(id),
    transaction_id UUID REFERENCES transactions(id),
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

-- Seed data for testing
INSERT INTO users (id, email, name, is_active) VALUES
    ('11111111-1111-1111-1111-111111111111', 'test@example.com', 'Test User', true),
    ('22222222-2222-2222-2222-222222222222', 'inactive@example.com', 'Inactive User', false);

INSERT INTO wallets (id, user_id, balance_cents) VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '11111111-1111-1111-1111-111111111111', 100000),
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', '22222222-2222-2222-2222-222222222222', 0);

INSERT INTO offerings (id, name, description, price_cents, is_active) VALUES
    ('cccccccc-cccc-cccc-cccc-cccccccccccc', 'Premium Plan', 'Monthly premium subscription', 9900, true),
    ('dddddddd-dddd-dddd-dddd-dddddddddddd', 'Basic Plan', 'Monthly basic subscription', 4900, true),
    ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'One-time Feature', 'Unlock special feature', 1999, true);
