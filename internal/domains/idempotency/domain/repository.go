package domain

import (
	"context"
)

// Repository defines the interface for idempotency record persistence.
type Repository interface {
	// Get retrieves an idempotency record by key.
	// Returns ErrNotFound if the record doesn't exist or is expired.
	Get(ctx context.Context, key string) (*IdempotencyRecord, error)

	// Create inserts a new idempotency record.
	// Returns ErrAlreadyExists if a record with the same key exists.
	Create(ctx context.Context, record *IdempotencyRecord) error

	// Update updates an existing idempotency record (typically to store the response).
	Update(ctx context.Context, record *IdempotencyRecord) error

	// Delete removes a single idempotency record by key.
	Delete(ctx context.Context, key string) error

	// DeleteExpiredBatch removes up to `limit` expired records.
	// Returns the number of deleted records.
	DeleteExpiredBatch(ctx context.Context, limit int) (int64, error)
}
