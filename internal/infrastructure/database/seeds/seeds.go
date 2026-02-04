package seeds

import (
	"context"
	"database/sql"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/config"
	"github.com/mancalvo/ssr-backend-draftea-challenge/pkg/logger"
)

func Run(ctx context.Context, db *sql.DB, cfg *config.Config) error {
	if cfg.Env != "development" {
		logger.Info("skipping seeds - not in development environment", "env", cfg.Env)
		return nil
	}

	runner := NewRunner()
	runner.AddSeed(&DevSeed{})

	logger.Info("running development seeds")
	return runner.Run(ctx, db)
}
