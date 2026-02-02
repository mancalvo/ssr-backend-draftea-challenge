package infrastructure

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/domain"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/database"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
)

type PostgresTransactionRepository struct {
	db database.Executor
}

func NewPostgresTransactionRepository(db database.Executor) *PostgresTransactionRepository {
	return &PostgresTransactionRepository{db: db}
}

// isUniqueViolation checks if error is a PostgreSQL unique constraint violation
func isUniqueViolation(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Code == "23505" // unique_violation
	}
	return false
}

func (r *PostgresTransactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	query := `
		INSERT INTO transactions (id, user_id, wallet_id, type, amount_cents, status, offering_id, provider_ref, idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.ExecContext(ctx, query,
		tx.ID,
		tx.UserID,
		tx.WalletID,
		tx.Type,
		tx.Amount,
		tx.Status,
		tx.OfferingID,
		tx.ProviderRef,
		tx.IdempotencyKey,
		tx.CreatedAt,
	)

	if err != nil && isUniqueViolation(err) {
		return apperrors.ErrAlreadyExists
	}

	return err
}

func (r *PostgresTransactionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.TransactionStatus) error {
	query := `UPDATE transactions SET status = $1 WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}

func (r *PostgresTransactionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	query := `
		SELECT id, user_id, wallet_id, type, amount_cents, status, offering_id, provider_ref, idempotency_key, created_at
		FROM transactions
		WHERE id = $1
	`

	var tx domain.Transaction
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tx.ID,
		&tx.UserID,
		&tx.WalletID,
		&tx.Type,
		&tx.Amount,
		&tx.Status,
		&tx.OfferingID,
		&tx.ProviderRef,
		&tx.IdempotencyKey,
		&tx.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &tx, nil
}

func (r *PostgresTransactionRepository) GetByUserAndIdempotencyKey(ctx context.Context, userID uuid.UUID, key string) (*domain.Transaction, error) {
	query := `
		SELECT id, user_id, wallet_id, type, amount_cents, status, offering_id, provider_ref, idempotency_key, created_at
		FROM transactions
		WHERE user_id = $1 AND idempotency_key = $2
	`

	var tx domain.Transaction
	err := r.db.QueryRowContext(ctx, query, userID, key).Scan(
		&tx.ID,
		&tx.UserID,
		&tx.WalletID,
		&tx.Type,
		&tx.Amount,
		&tx.Status,
		&tx.OfferingID,
		&tx.ProviderRef,
		&tx.IdempotencyKey,
		&tx.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &tx, nil
}

func (r *PostgresTransactionRepository) GetByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) (*domain.PaginatedTransactions, error) {
	// Count total
	countQuery := `SELECT COUNT(*) FROM transactions WHERE user_id = $1`
	var totalCount int
	if err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&totalCount); err != nil {
		return nil, err
	}

	// Calculate pagination
	offset := (page - 1) * pageSize
	totalPages := (totalCount + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	// Get transactions
	query := `
		SELECT id, user_id, wallet_id, type, amount_cents, status, offering_id, provider_ref, idempotency_key, created_at
		FROM transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, userID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []domain.Transaction
	for rows.Next() {
		var tx domain.Transaction
		if err := rows.Scan(
			&tx.ID,
			&tx.UserID,
			&tx.WalletID,
			&tx.Type,
			&tx.Amount,
			&tx.Status,
			&tx.OfferingID,
			&tx.ProviderRef,
			&tx.IdempotencyKey,
			&tx.CreatedAt,
		); err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &domain.PaginatedTransactions{
		Transactions: transactions,
		Page:         page,
		PageSize:     pageSize,
		TotalCount:   totalCount,
		TotalPages:   totalPages,
	}, nil
}
