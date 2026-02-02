package infrastructure

import (
	"context"
	"database/sql"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/users/domain"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/database"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
)

type PostgresUserRepository struct {
	db database.Executor
}

func NewPostgresUserRepository(db database.Executor) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, email, name, is_active, created_at
		FROM users
		WHERE id = $1
	`

	var user domain.User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.IsActive,
		&user.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *PostgresUserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, email, name, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.Email,
		user.Name,
		user.IsActive,
		user.CreatedAt,
	)

	return err
}
