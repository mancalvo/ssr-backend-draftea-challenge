package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/idempotency/domain"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/database"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
)

type PostgresRepository struct {
	db database.Executor
}

func NewPostgresRepository(db database.Executor) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // unique_violation
	}
	return false
}

func (r *PostgresRepository) Get(ctx context.Context, key string) (*domain.IdempotencyRecord, error) {
	query := `
		SELECT key, request_hash, status_code, response_body, content_type, created_at, expires_at
		FROM idempotency_records
		WHERE key = $1
	`

	var record domain.IdempotencyRecord
	var responseBody []byte

	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&record.Key,
		&record.RequestHash,
		&record.StatusCode,
		&responseBody,
		&record.ContentType,
		&record.CreatedAt,
		&record.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	record.ResponseBody = responseBody

	// Lazy cleanup: if expired, delete and return not found
	if time.Now().After(record.ExpiresAt) {
		_ = r.Delete(ctx, key)
		return nil, apperrors.ErrNotFound
	}

	return &record, nil
}

func (r *PostgresRepository) Create(ctx context.Context, record *domain.IdempotencyRecord) error {
	query := `
		INSERT INTO idempotency_records (key, request_hash, status_code, response_body, content_type, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		record.Key,
		record.RequestHash,
		record.StatusCode,
		record.ResponseBody,
		record.ContentType,
		record.CreatedAt,
		record.ExpiresAt,
	)

	if err != nil && isUniqueViolation(err) {
		return apperrors.ErrAlreadyExists
	}

	return err
}

func (r *PostgresRepository) Update(ctx context.Context, record *domain.IdempotencyRecord) error {
	query := `
		UPDATE idempotency_records 
		SET status_code = $1, response_body = $2, content_type = $3
		WHERE key = $4
	`

	result, err := r.db.ExecContext(ctx, query,
		record.StatusCode,
		record.ResponseBody,
		record.ContentType,
		record.Key,
	)
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

func (r *PostgresRepository) Delete(ctx context.Context, key string) error {
	query := `DELETE FROM idempotency_records WHERE key = $1`
	_, err := r.db.ExecContext(ctx, query, key)
	return err
}

func (r *PostgresRepository) DeleteExpiredBatch(ctx context.Context, limit int) (int64, error) {
	// Use a subquery to limit the number of deleted rows efficiently
	query := `
		DELETE FROM idempotency_records 
		WHERE key IN (
			SELECT key FROM idempotency_records 
			WHERE expires_at < NOW() 
			LIMIT $1
		)
	`

	result, err := r.db.ExecContext(ctx, query, limit)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}
