package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
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

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // unique_violation
	}
	return false
}

// getExecutor returns the transaction from context if present, otherwise the default db
func (r *PostgresTransactionRepository) getExecutor(ctx context.Context) database.Executor {
	return database.ExecutorFromContext(ctx, r.db)
}

func (r *PostgresTransactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	query := `
		INSERT INTO transactions (id, user_id, wallet_id, type, amount_cents, status, offering_id, provider_ref, idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.getExecutor(ctx).ExecContext(ctx, query,
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

func (r *PostgresTransactionRepository) UpdateStatus(ctx context.Context, id string, status domain.TransactionStatus) error {
	query := `UPDATE transactions SET status = $1 WHERE id = $2`

	result, err := r.getExecutor(ctx).ExecContext(ctx, query, status, id)
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

func (r *PostgresTransactionRepository) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	query := `
		SELECT id, user_id, wallet_id, type, amount_cents, status, offering_id, provider_ref, idempotency_key, created_at
		FROM transactions
		WHERE id = $1
	`

	var tx domain.Transaction
	err := r.getExecutor(ctx).QueryRowContext(ctx, query, id).Scan(
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

func (r *PostgresTransactionRepository) GetByUserID(ctx context.Context, userID string, page, pageSize int) (*domain.PaginatedTransactions, error) {
	// Count total
	countQuery := `SELECT COUNT(*) FROM transactions WHERE user_id = $1`
	exec := r.getExecutor(ctx)
	var totalCount int
	if err := exec.QueryRowContext(ctx, countQuery, userID).Scan(&totalCount); err != nil {
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

	rows, err := exec.QueryContext(ctx, query, userID, pageSize, offset)
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
