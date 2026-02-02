package domain

import (
	"time"
)

type User struct {
	ID        string
	Email     string
	Name      string
	IsActive  bool
	CreatedAt time.Time
}
