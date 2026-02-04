-- Drop indexes first
DROP INDEX IF EXISTS idx_entitlements_user_offering_active;
DROP INDEX IF EXISTS idx_entitlements_transaction;
DROP INDEX IF EXISTS idx_entitlements_offering;
DROP INDEX IF EXISTS idx_entitlements_user;

-- Entitlements table
DROP TABLE entitlements;
