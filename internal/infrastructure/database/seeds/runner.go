package seeds

import (
	"context"
	"database/sql"

	"github.com/mancalvo/ssr-backend-draftea-challenge/pkg/logger"
)

type Seed interface {
	Name() string
	Run(ctx context.Context, db *sql.DB) error
}

type Runner struct {
	seeds []Seed
}

func NewRunner() *Runner {
	return &Runner{seeds: []Seed{}}
}

func (r *Runner) AddSeed(seed Seed) {
	r.seeds = append(r.seeds, seed)
}

func (r *Runner) Run(ctx context.Context, db *sql.DB) error {
	for _, seed := range r.seeds {
		logger.Info("running seed", "name", seed.Name())
		if err := seed.Run(ctx, db); err != nil {
			return err
		}
		logger.Info("seed completed", "name", seed.Name())
	}
	logger.Info("all seeds completed successfully")
	return nil
}
