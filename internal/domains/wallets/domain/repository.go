package domain

import (
	"context"

	"github.com/google/uuid"
)

type WalletRepository interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (*Wallet, error)
	Credit(ctx context.Context, walletID uuid.UUID, amount int64) error
	Debit(ctx context.Context, walletID uuid.UUID, amount int64) error
	Create(ctx context.Context, wallet *Wallet) error
}
