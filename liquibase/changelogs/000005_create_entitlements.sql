--liquibase formatted sql

--changeset mancalvo:000005-create-entitlements
--comment: User access to offerings
CREATE TABLE entitlements (
    id CHAR(20) PRIMARY KEY,
    user_id CHAR(20) REFERENCES users(id),
    offering_id CHAR(20) REFERENCES offerings(id),
    transaction_id CHAR(20) REFERENCES transactions(id),
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    granted_at TIMESTAMPTZ DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

--changeset mancalvo:000005-create-entitlements-indexes
--comment: Indexes for entitlements
CREATE INDEX idx_entitlements_user ON entitlements(user_id);
CREATE INDEX idx_entitlements_offering ON entitlements(offering_id);
CREATE INDEX idx_entitlements_transaction ON entitlements(transaction_id);
CREATE UNIQUE INDEX idx_entitlements_user_offering_active 
ON entitlements (user_id, offering_id) 
WHERE status = 'active';

--rollback DROP INDEX IF EXISTS idx_entitlements_user_offering_active;
--rollback DROP INDEX IF EXISTS idx_entitlements_transaction;
--rollback DROP INDEX IF EXISTS idx_entitlements_offering;
--rollback DROP INDEX IF EXISTS idx_entitlements_user;
--rollback DROP TABLE entitlements;
