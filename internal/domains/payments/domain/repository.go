package domain

import (
	"context"

	"github.com/google/uuid"
)

type TransactionRepository interface {
	Create(ctx context.Context, tx *Transaction) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status TransactionStatus) error
	GetByID(ctx context.Context, id uuid.UUID) (*Transaction, error)
	GetByUserAndIdempotencyKey(ctx context.Context, userID uuid.UUID, key string) (*Transaction, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) (*PaginatedTransactions, error)
}
