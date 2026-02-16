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

-- Indexes for transactions
CREATE INDEX idx_transactions_user ON transactions(user_id, created_at DESC);
CREATE INDEX idx_transactions_wallet ON transactions(wallet_id);
