package domain

import (
	"context"
)

type WalletService interface {
	GetBalance(ctx context.Context, userID string) (balanceCents int64, walletID string, err error)
	Credit(ctx context.Context, userID string, amountCents int64) error
	Debit(ctx context.Context, userID string, amountCents int64) error
}

type UserService interface {
	IsActive(ctx context.Context, userID string) (bool, error)
}

type EntitlementService interface {
	GrantAccess(ctx context.Context, userID, offeringID, transactionID string) error
	RevokeAccess(ctx context.Context, userID, offeringID string) error
	HasActiveAccess(ctx context.Context, userID, offeringID string) (bool, error)
	GetActiveEntitlementForOffering(ctx context.Context, userID, offeringID string) (transactionID string, err error)
}

type OfferingService interface {
	GetPrice(ctx context.Context, offeringID string) (priceCents int64, err error)
	IsAvailable(ctx context.Context, offeringID string) (bool, error)
}
