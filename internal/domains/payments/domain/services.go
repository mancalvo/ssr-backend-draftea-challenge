package domain

import (
	"context"

	"github.com/google/uuid"
)

type WalletService interface {
	GetBalance(ctx context.Context, userID uuid.UUID) (balanceCents int64, walletID uuid.UUID, err error)
	Credit(ctx context.Context, userID uuid.UUID, amountCents int64) error
	Debit(ctx context.Context, userID uuid.UUID, amountCents int64) error
}

type UserService interface {
	IsActive(ctx context.Context, userID uuid.UUID) (bool, error)
}

type EntitlementService interface {
	GrantAccess(ctx context.Context, userID, offeringID, transactionID uuid.UUID) error
	RevokeAccess(ctx context.Context, userID, offeringID uuid.UUID) error
	HasActiveAccess(ctx context.Context, userID, offeringID uuid.UUID) (bool, error)
	GetActiveEntitlementForOffering(ctx context.Context, userID, offeringID uuid.UUID) (transactionID uuid.UUID, err error)
}

type OfferingService interface {
	GetPrice(ctx context.Context, offeringID uuid.UUID) (priceCents int64, err error)
	IsAvailable(ctx context.Context, offeringID uuid.UUID) (bool, error)
}
