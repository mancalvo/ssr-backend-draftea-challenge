package infrastructure

import (
	"context"
	"database/sql"
	"time"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/wallets/domain"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/database"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
)

type PostgresWalletRepository struct {
	db database.Executor
}

func NewPostgresWalletRepository(db database.Executor) *PostgresWalletRepository {
	return &PostgresWalletRepository{db: db}
}

// getExecutor returns the transaction from context if present, otherwise the default db
func (r *PostgresWalletRepository) getExecutor(ctx context.Context) database.Executor {
	return database.ExecutorFromContext(ctx, r.db)
}

func (r *PostgresWalletRepository) GetByUserID(ctx context.Context, userID string) (*domain.Wallet, error) {
	query := `
		SELECT id, user_id, balance_cents, updated_at
		FROM wallets
		WHERE user_id = $1
	`

	var wallet domain.Wallet
	err := r.getExecutor(ctx).QueryRowContext(ctx, query, userID).Scan(
		&wallet.ID,
		&wallet.UserID,
		&wallet.Balance,
		&wallet.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &wallet, nil
}

// Credit atomically adds amount to wallet balance
func (r *PostgresWalletRepository) Credit(ctx context.Context, walletID string, amount int64) error {
	query := `
		UPDATE wallets
		SET balance_cents = balance_cents + $1, updated_at = $2
		WHERE id = $3
	`

	result, err := r.getExecutor(ctx).ExecContext(ctx, query, amount, time.Now(), walletID)
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

// Debit atomically subtracts amount from wallet balance
// Returns ErrInsufficientFunds if balance would go negative
func (r *PostgresWalletRepository) Debit(ctx context.Context, walletID string, amount int64) error {
	query := `
		UPDATE wallets
		SET balance_cents = balance_cents - $1, updated_at = $2
		WHERE id = $3 AND balance_cents >= $1
	`

	exec := r.getExecutor(ctx)
	result, err := exec.ExecContext(ctx, query, amount, time.Now(), walletID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		// Could be: wallet not found OR insufficient funds
		var balance int64
		checkQuery := `SELECT balance_cents FROM wallets WHERE id = $1`
		err := exec.QueryRowContext(ctx, checkQuery, walletID).Scan(&balance)
		if err == sql.ErrNoRows {
			return apperrors.ErrNotFound
		}
		if err != nil {
			return err
		}
		return apperrors.ErrInsufficientFunds
	}

	return nil
}

func (r *PostgresWalletRepository) Create(ctx context.Context, wallet *domain.Wallet) error {
	query := `
		INSERT INTO wallets (id, user_id, balance_cents, updated_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.getExecutor(ctx).ExecContext(ctx, query,
		wallet.ID,
		wallet.UserID,
		wallet.Balance,
		wallet.UpdatedAt,
	)

	return err
}

