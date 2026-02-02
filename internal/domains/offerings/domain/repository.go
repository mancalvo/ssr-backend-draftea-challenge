package domain

import (
	"context"

	"github.com/google/uuid"
)

type OfferingRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Offering, error)
	List(ctx context.Context) ([]Offering, error)
}

type EntitlementRepository interface {
	Create(ctx context.Context, ent *Entitlement) error
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]Entitlement, error)
	GetByTransactionID(ctx context.Context, txID uuid.UUID) (*Entitlement, error)
	Revoke(ctx context.Context, id uuid.UUID) error
}
