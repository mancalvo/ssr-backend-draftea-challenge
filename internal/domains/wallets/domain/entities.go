package domain

import (
	"time"

	"github.com/google/uuid"
)

type Wallet struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Balance   int64 // cents (MXN)
	UpdatedAt time.Time
}
