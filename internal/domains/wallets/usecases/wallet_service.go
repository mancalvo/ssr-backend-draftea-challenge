package usecases

import (
	"context"

	"github.com/google/uuid"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/wallets/domain"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
)

type WalletService interface {
	GetBalance(ctx context.Context, userID uuid.UUID) (balanceCents int64, walletID uuid.UUID, err error)
	Credit(ctx context.Context, userID uuid.UUID, amountCents int64) error
	Debit(ctx context.Context, userID uuid.UUID, amountCents int64) error
}

type walletService struct {
	walletRepo domain.WalletRepository
}

func NewWalletService(walletRepo domain.WalletRepository) WalletService {
	return &walletService{walletRepo: walletRepo}
}

func (s *walletService) GetBalance(ctx context.Context, userID uuid.UUID) (int64, uuid.UUID, error) {
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return 0, uuid.Nil, err
	}
	return wallet.Balance, wallet.ID, nil
}

func (s *walletService) Credit(ctx context.Context, userID uuid.UUID, amountCents int64) error {
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}
	return s.walletRepo.Credit(ctx, wallet.ID, amountCents)
}

func (s *walletService) Debit(ctx context.Context, userID uuid.UUID, amountCents int64) error {
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if wallet.Balance < amountCents {
		return apperrors.ErrInsufficientFunds
	}
	return s.walletRepo.Debit(ctx, wallet.ID, amountCents)
}
