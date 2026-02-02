package domain

import (
	"time"
)

type Wallet struct {
	ID        string
	UserID    string
	Balance   int64 // cents (MXN)
	UpdatedAt time.Time
}
