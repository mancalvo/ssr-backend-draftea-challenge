package database

import (
	"fmt"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/config"
	"github.com/mancalvo/ssr-backend-draftea-challenge/migrations"
	"github.com/mancalvo/ssr-backend-draftea-challenge/pkg/logger"
)

func MigrateUp(cfg config.DatabaseConfig) error {
	dbURL := buildPostgresURL(cfg)

	sourceDriver, err := iofs.New(migrations.MigrationsFS, ".")
	if err != nil {
		return fmt.Errorf("failed to create migration source driver: %w", err)
	}

	m, err := migrate.NewWithSourceInstance(
		"iofs",
		sourceDriver,
		dbURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Info("migrations completed successfully")
	return nil
}

func MigrateDown(cfg config.DatabaseConfig) error {
	dbURL := buildPostgresURL(cfg)

	sourceDriver, err := iofs.New(migrations.MigrationsFS, ".")
	if err != nil {
		return fmt.Errorf("failed to create migration source driver: %w", err)
	}

	m, err := migrate.NewWithSourceInstance(
		"iofs",
		sourceDriver,
		dbURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run down migrations: %w", err)
	}

	logger.Info("down migrations completed successfully")
	return nil
}

func buildPostgresURL(cfg config.DatabaseConfig) string {
	return "postgres://" + cfg.User +
		":" + cfg.Password +
		"@" + cfg.Host +
		":" + strconv.Itoa(cfg.Port) +
		"/" + cfg.DBName +
		"?sslmode=" + cfg.SSLMode
}
