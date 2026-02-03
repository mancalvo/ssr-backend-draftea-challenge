package deposit

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/domain"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
	"github.com/rs/xid"
)

type UseCase interface {
	Execute(ctx context.Context, userID string, amount int64, cardToken string, idempotencyKey *string) (*domain.Transaction, error)
}

type PaymentDepositUseCase struct {
	txRepo    domain.TransactionRepository
	walletSvc domain.WalletService
	userSvc   domain.UserService
	provider  domain.PaymentProvider
}

func New(
	txRepo domain.TransactionRepository,
	walletSvc domain.WalletService,
	userSvc domain.UserService,
	provider domain.PaymentProvider,
) *PaymentDepositUseCase {
	return &PaymentDepositUseCase{
		txRepo:    txRepo,
		walletSvc: walletSvc,
		userSvc:   userSvc,
		provider:  provider,
	}
}

// Execute processes a card payment and credits the wallet.
// Uses "Record First" pattern: creates PENDING transaction before calling provider.
// This ensures we always have a record for reconciliation if provider call succeeds but DB fails.
// If idempotencyKey is provided, returns cached result for duplicate requests.
func (uc *PaymentDepositUseCase) Execute(ctx context.Context, userID string, amount int64, cardToken string, idempotencyKey *string) (*domain.Transaction, error) {
	// Check idempotency key first (scoped by user for security)
	if idempotencyKey != nil && *idempotencyKey != "" {
		existing, err := uc.txRepo.GetByUserAndIdempotencyKey(ctx, userID, *idempotencyKey)
		if err == nil {
			// Found existing transaction - return it (idempotent response)
			return existing, nil
		}
		if !errors.Is(err, apperrors.ErrNotFound) {
			return nil, err
		}
		// Not found - proceed with processing
	}

	if amount <= 0 {
		return nil, apperrors.ErrInvalidInput
	}

	isActive, err := uc.userSvc.IsActive(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !isActive {
		return nil, apperrors.ErrUserNotActive
	}

	_, walletID, err := uc.walletSvc.GetBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	txID := xid.New().String()
	tx := &domain.Transaction{
		ID:             txID,
		UserID:         userID,
		WalletID:       walletID,
		Type:           domain.TxDeposit,
		Amount:         amount,
		Status:         domain.TxPending,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now(),
	}

	if err := uc.txRepo.Create(ctx, tx); err != nil {
		// Repository now returns ErrAlreadyExists for duplicate idempotency key
		if errors.Is(err, apperrors.ErrAlreadyExists) && idempotencyKey != nil {
			existing, _ := uc.txRepo.GetByUserAndIdempotencyKey(ctx, userID, *idempotencyKey)
			if existing != nil {
				return existing, nil
			}
		}
		return nil, err
	}

	// Generate hashed provider key: sha256(userID + idempotencyKey) for privacy
	providerKey := ""
	if idempotencyKey != nil && *idempotencyKey != "" {
		hash := sha256.Sum256([]byte(userID + *idempotencyKey))
		providerKey = fmt.Sprintf("%x", hash)[:32] // First 32 hex chars
	} else {
		// Fallback: use transaction ID if no idempotency key
		providerKey = tx.ID
	}

	providerResp, err := uc.provider.ProcessPayment(ctx, amount, cardToken, providerKey)
	if err != nil {
		_ = uc.txRepo.UpdateStatus(ctx, tx.ID, domain.TxProviderFailed)
		return nil, apperrors.ErrProviderError
	}

	if !providerResp.Success {
		_ = uc.txRepo.UpdateStatus(ctx, tx.ID, domain.TxProviderFailed)
		return nil, apperrors.NewProviderDeclined(providerResp.ErrorMessage)
	}

	// Provider succeeded - mark as charged
	tx.ProviderRef = &providerResp.Reference
	_ = uc.txRepo.UpdateStatus(ctx, tx.ID, domain.TxProviderCharged)

	if err := uc.walletSvc.Credit(ctx, userID, amount); err != nil {
		// Provider charged but wallet credit failed - mark for reconciliation
		_ = uc.txRepo.UpdateStatus(ctx, tx.ID, domain.TxWalletFailed)
		// Return the transaction with wallet_failed status (handler will return 202)
		tx.Status = domain.TxWalletFailed
		return tx, apperrors.ErrWalletCreditFailed
	}

	// Fully completed
	if err := uc.txRepo.UpdateStatus(ctx, tx.ID, domain.TxCompleted); err != nil {
		// Already credited, just status update failed - return success anyway
		tx.Status = domain.TxProviderCharged
		return tx, nil
	}

	tx.Status = domain.TxCompleted
	return tx, nil
}
