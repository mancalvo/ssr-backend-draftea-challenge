package domain

import (
	"time"

	"github.com/google/uuid"
)

type TransactionType string

const (
	TxDeposit  TransactionType = "deposit"  // Card → Wallet
	TxPurchase TransactionType = "purchase" // Wallet → Offering
	TxRefund   TransactionType = "refund"   // Offering → Wallet
)

type TransactionStatus string

const (
	TxPending         TransactionStatus = "pending"          // Created, not yet processed
	TxProviderCharged TransactionStatus = "provider_charged" // Provider succeeded, wallet pending
	TxCompleted       TransactionStatus = "completed"        // Fully completed
	TxProviderFailed  TransactionStatus = "provider_failed"  // Provider declined/error
	TxWalletFailed    TransactionStatus = "wallet_failed"    // Provider charged, wallet credit failed (needs reconciliation!)

	// Refund-specific statuses
	TxWalletCredited     TransactionStatus = "wallet_credited"     // Refund: wallet credited, revoke pending
	TxEntitlementRevoked TransactionStatus = "entitlement_revoked" // Refund: entitlement revoked, credit pending
	TxCreditFailed       TransactionStatus = "credit_failed"       // Refund: wallet credit failed (needs reconciliation)
	TxRevokeFailed       TransactionStatus = "revoke_failed"       // Refund: entitlement revoke failed (needs reconciliation)
)

type Transaction struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	WalletID       uuid.UUID
	Type           TransactionType
	Amount         int64 // always positive, in cents
	Status         TransactionStatus
	OfferingID     *uuid.UUID // for purchase/refund
	ProviderRef    *string    // for deposits
	IdempotencyKey *string    // client-provided key for deduplication
	CreatedAt      time.Time
}

type PaginatedTransactions struct {
	Transactions []Transaction
	Page         int
	PageSize     int
	TotalCount   int
	TotalPages   int
}
