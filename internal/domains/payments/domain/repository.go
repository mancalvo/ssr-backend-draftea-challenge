package domain

import (
	"context"
)

type TransactionRepository interface {
	Create(ctx context.Context, tx *Transaction) error
	UpdateStatus(ctx context.Context, id string, status TransactionStatus) error
	GetByID(ctx context.Context, id string) (*Transaction, error)
	GetByUserAndIdempotencyKey(ctx context.Context, userID string, key string) (*Transaction, error)
	GetByUserID(ctx context.Context, userID string, page, pageSize int) (*PaginatedTransactions, error)
}
