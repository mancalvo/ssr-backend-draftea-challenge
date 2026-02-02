package domain

import (
	"context"
)

type WalletRepository interface {
	GetByUserID(ctx context.Context, userID string) (*Wallet, error)
	Credit(ctx context.Context, walletID string, amount int64) error
	Debit(ctx context.Context, walletID string, amount int64) error
	Create(ctx context.Context, wallet *Wallet) error
}
