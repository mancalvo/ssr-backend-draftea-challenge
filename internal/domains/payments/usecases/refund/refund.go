package refund

import (
	"context"
	"errors"
	"time"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/domain"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/shared/uow"
	"github.com/rs/xid"
)

type UseCase interface {
	Execute(ctx context.Context, userID, offeringID string, idempotencyKey *string) (*domain.Transaction, error)
}

type PaymentRefundUseCase struct {
	txRunner   uow.TransactionRunner
	txRepo     domain.TransactionRepository
	walletSvc  domain.WalletService
	entitleSvc domain.EntitlementService
}

func New(
	txRunner uow.TransactionRunner,
	txRepo domain.TransactionRepository,
	walletSvc domain.WalletService,
	entitleSvc domain.EntitlementService,
) *PaymentRefundUseCase {
	return &PaymentRefundUseCase{
		txRunner:   txRunner,
		txRepo:     txRepo,
		walletSvc:  walletSvc,
		entitleSvc: entitleSvc,
	}
}

// Execute reverses a purchase: credits wallet and revokes entitlement
// Uses transaction to ensure atomicity of credit + revoke operations
func (uc *PaymentRefundUseCase) Execute(ctx context.Context, userID, offeringID string, idempotencyKey *string) (*domain.Transaction, error) {
	// Check idempotency key first (scoped by user for security) - outside transaction
	if idempotencyKey != nil && *idempotencyKey != "" {
		existing, err := uc.txRepo.GetByUserAndIdempotencyKey(ctx, userID, *idempotencyKey)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, apperrors.ErrNotFound) {
			return nil, err
		}
	}

	// Get the original transaction ID for this user+offering via entitlement
	originalTxID, err := uc.entitleSvc.GetActiveEntitlementForOffering(ctx, userID, offeringID)
	if err != nil {
		return nil, err // No active entitlement found
	}

	// Get the original purchase transaction for the amount
	originalTx, err := uc.txRepo.GetByID(ctx, originalTxID)
	if err != nil {
		return nil, err
	}

	_, walletID, err := uc.walletSvc.GetBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Prepare refund transaction
	refundTxID := xid.New().String()
	refundTx := &domain.Transaction{
		ID:             refundTxID,
		UserID:         userID,
		WalletID:       walletID,
		Type:           domain.TxRefund,
		Amount:         originalTx.Amount,
		Status:         domain.TxCompleted, // Will be committed as completed if transaction succeeds
		OfferingID:     &offeringID,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now(),
	}

	// Critical operations wrapped in transaction for atomicity
	err = uc.txRunner.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Create refund transaction record
		if err := uc.txRepo.Create(txCtx, refundTx); err != nil {
			if errors.Is(err, apperrors.ErrAlreadyExists) && idempotencyKey != nil {
				return err // Idempotency collision - will be handled after transaction
			}
			return err
		}

		// Credit wallet
		if err := uc.walletSvc.Credit(txCtx, userID, originalTx.Amount); err != nil {
			return err
		}

		// Revoke entitlement
		if err := uc.entitleSvc.RevokeAccess(txCtx, userID, offeringID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		// Handle idempotency collision that happened inside transaction
		if errors.Is(err, apperrors.ErrAlreadyExists) && idempotencyKey != nil {
			existing, _ := uc.txRepo.GetByUserAndIdempotencyKey(ctx, userID, *idempotencyKey)
			if existing != nil {
				return existing, nil
			}
		}
		return nil, err
	}

	return refundTx, nil
}
