package domain

import (
	"time"

	"github.com/google/uuid"
)

type Offering struct {
	ID           uuid.UUID
	Name         string
	Description  string
	PriceCents   int64
	DurationDays *int // nil = perpetual
	IsActive     bool
	CreatedAt    time.Time
}

type EntitlementStatus string

const (
	EntitlementActive  EntitlementStatus = "active"
	EntitlementRevoked EntitlementStatus = "revoked"
)

type Entitlement struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	OfferingID    uuid.UUID
	TransactionID uuid.UUID
	Status        EntitlementStatus
	GrantedAt     time.Time
	RevokedAt     *time.Time
}
