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

-- Indexes for entitlements
CREATE INDEX idx_entitlements_user ON entitlements(user_id);
CREATE INDEX idx_entitlements_offering ON entitlements(offering_id);
CREATE INDEX idx_entitlements_transaction ON entitlements(transaction_id);

-- Unique constraint: one active entitlement per user+offering
CREATE UNIQUE INDEX idx_entitlements_user_offering_active 
ON entitlements (user_id, offering_id) 
WHERE status = 'active';
