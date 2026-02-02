package domain

import (
	"context"
)

type OfferingRepository interface {
	GetByID(ctx context.Context, id string) (*Offering, error)
	List(ctx context.Context) ([]Offering, error)
}

type EntitlementRepository interface {
	Create(ctx context.Context, ent *Entitlement) error
	GetByUserID(ctx context.Context, userID string) ([]Entitlement, error)
	GetByTransactionID(ctx context.Context, txID string) (*Entitlement, error)
	Revoke(ctx context.Context, id string) error
}
