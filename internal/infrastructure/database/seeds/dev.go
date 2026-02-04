package seeds

import (
	"context"
	"database/sql"

	"github.com/mancalvo/ssr-backend-draftea-challenge/pkg/logger"
)

type DevSeed struct{}

func (s *DevSeed) Name() string {
	return "dev_data"
}

func (s *DevSeed) Run(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.seedUsers(ctx, tx); err != nil {
		return err
	}
	if err := s.seedWallets(ctx, tx); err != nil {
		return err
	}
	if err := s.seedOfferings(ctx, tx); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *DevSeed) seedUsers(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, email, name, is_active)
		VALUES 
		    ('d60g19m0u7j2796eeac0', 'test@example.com', 'Test User', true),
		    ('d60g19m0u7j2796eeacg', 'inactive@example.com', 'Inactive User', false)
		ON CONFLICT (id) DO NOTHING
	`)
	if err != nil {
		logger.Error("failed to seed users", "error", err)
		return err
	}
	return nil
}

func (s *DevSeed) seedWallets(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO wallets (id, user_id, balance_cents)
		VALUES 
		    ('d60g19m0u7j2796eead0', 'd60g19m0u7j2796eeac0', 100000),
		    ('d60g19m0u7j2796eeadg', 'd60g19m0u7j2796eeacg', 0)
		ON CONFLICT (user_id) DO NOTHING
	`)
	if err != nil {
		logger.Error("failed to seed wallets", "error", err)
		return err
	}
	return nil
}

func (s *DevSeed) seedOfferings(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO offerings (id, name, description, price_cents, is_active)
		VALUES 
		    ('d60g19m0u7j2796eeae0', 'Premium Plan', 'Monthly premium subscription', 9900, true),
		    ('d60g19m0u7j2796eeaeg', 'Basic Plan', 'Monthly basic subscription', 4900, true),
		    ('d60g19m0u7j2796eeaf0', 'One-time Feature', 'Unlock special feature', 1999, true)
		ON CONFLICT (id) DO NOTHING
	`)
	if err != nil {
		logger.Error("failed to seed offerings", "error", err)
		return err
	}
	return nil
}
