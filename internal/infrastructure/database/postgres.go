package database

import (
	"context"
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/config"
	"github.com/mancalvo/ssr-backend-draftea-challenge/pkg/logger"
)

// DB wraps sql.DB and provides transaction support
type DB struct {
	*sql.DB
}

// Tx wraps sql.Tx for use in repositories
type Tx struct {
	*sql.Tx
}

// Executor interface allows repositories to work with both DB and Tx
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

func NewPostgresConnection(cfg config.DatabaseConfig) (*DB, error) {
	dsn := cfg.ConnectionString()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	logger.Info("connected to database", "host", cfg.Host, "database", cfg.DBName)

	return &DB{db}, nil
}

// BeginTx starts a new transaction
func (db *DB) BeginTx(ctx context.Context) (*Tx, error) {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{tx}, nil
}

// WithTx executes a function within a transaction
// Commits on success, rolls back on error or panic
func (db *DB) WithTx(ctx context.Context, fn func(tx *Tx) error) error {
	tx, err := db.BeginTx(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p) // re-throw after rollback
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			logger.Error("tx rollback failed", "error", rbErr)
		}
		return err
	}

	return tx.Commit()
}
