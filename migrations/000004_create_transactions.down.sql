-- Drop indexes first
DROP INDEX IF EXISTS idx_transactions_user_idempotency_key;
DROP INDEX IF EXISTS idx_transactions_wallet;
DROP INDEX IF EXISTS idx_transactions_user;

-- Transactions table
DROP TABLE transactions;
