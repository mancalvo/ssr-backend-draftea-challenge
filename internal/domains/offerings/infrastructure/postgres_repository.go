package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/offerings/domain"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/database"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
)

// isUniqueViolation checks if error is a PostgreSQL unique constraint violation
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // unique_violation
	}
	return false
}

// Offering Repository

type PostgresOfferingRepository struct {
	db database.Executor
}

func NewPostgresOfferingRepository(db database.Executor) *PostgresOfferingRepository {
	return &PostgresOfferingRepository{db: db}
}

func (r *PostgresOfferingRepository) GetByID(ctx context.Context, id string) (*domain.Offering, error) {
	query := `
		SELECT id, name, description, price_cents, duration_days, is_active, created_at
		FROM offerings
		WHERE id = $1
	`

	var offering domain.Offering
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&offering.ID,
		&offering.Name,
		&offering.Description,
		&offering.PriceCents,
		&offering.DurationDays,
		&offering.IsActive,
		&offering.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &offering, nil
}

func (r *PostgresOfferingRepository) List(ctx context.Context) ([]domain.Offering, error) {
	query := `
		SELECT id, name, description, price_cents, duration_days, is_active, created_at
		FROM offerings
		WHERE is_active = true
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var offerings []domain.Offering
	for rows.Next() {
		var o domain.Offering
		if err := rows.Scan(
			&o.ID,
			&o.Name,
			&o.Description,
			&o.PriceCents,
			&o.DurationDays,
			&o.IsActive,
			&o.CreatedAt,
		); err != nil {
			return nil, err
		}
		offerings = append(offerings, o)
	}

	return offerings, rows.Err()
}

// Entitlement Repository

type PostgresEntitlementRepository struct {
	db database.Executor
}

func NewPostgresEntitlementRepository(db database.Executor) *PostgresEntitlementRepository {
	return &PostgresEntitlementRepository{db: db}
}

func (r *PostgresEntitlementRepository) Create(ctx context.Context, ent *domain.Entitlement) error {
	query := `
		INSERT INTO entitlements (id, user_id, offering_id, transaction_id, status, granted_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		ent.ID,
		ent.UserID,
		ent.OfferingID,
		ent.TransactionID,
		ent.Status,
		ent.GrantedAt,
		ent.RevokedAt,
	)

	if err != nil && isUniqueViolation(err) {
		return apperrors.ErrAlreadyOwned
	}

	return err
}

func (r *PostgresEntitlementRepository) GetByUserID(ctx context.Context, userID string) ([]domain.Entitlement, error) {
	query := `
		SELECT id, user_id, offering_id, transaction_id, status, granted_at, revoked_at
		FROM entitlements
		WHERE user_id = $1
		ORDER BY granted_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entitlements []domain.Entitlement
	for rows.Next() {
		var e domain.Entitlement
		if err := rows.Scan(
			&e.ID,
			&e.UserID,
			&e.OfferingID,
			&e.TransactionID,
			&e.Status,
			&e.GrantedAt,
			&e.RevokedAt,
		); err != nil {
			return nil, err
		}
		entitlements = append(entitlements, e)
	}

	return entitlements, rows.Err()
}

func (r *PostgresEntitlementRepository) GetByTransactionID(ctx context.Context, txID string) (*domain.Entitlement, error) {
	query := `
		SELECT id, user_id, offering_id, transaction_id, status, granted_at, revoked_at
		FROM entitlements
		WHERE transaction_id = $1
	`

	var e domain.Entitlement
	err := r.db.QueryRowContext(ctx, query, txID).Scan(
		&e.ID,
		&e.UserID,
		&e.OfferingID,
		&e.TransactionID,
		&e.Status,
		&e.GrantedAt,
		&e.RevokedAt,
	)

	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &e, nil
}

func (r *PostgresEntitlementRepository) Revoke(ctx context.Context, id string) error {
	query := `
		UPDATE entitlements
		SET status = $1, revoked_at = $2
		WHERE id = $3
	`

	result, err := r.db.ExecContext(ctx, query, domain.EntitlementRevoked, time.Now(), id)
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
