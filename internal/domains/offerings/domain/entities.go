package domain

import (
	"time"
)

type Offering struct {
	ID           string
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
	ID            string
	UserID        string
	OfferingID    string
	TransactionID string
	Status        EntitlementStatus
	GrantedAt     time.Time
	RevokedAt     *time.Time
}
