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

// UseCase defines the interface for deposit operations.
type UseCase interface {
	Execute(ctx context.Context, userID string, amount int64, cardToken string, idempotencyKey *string) (*domain.Transaction, error)
}

// PaymentDepositUseCase orchestrates deposit processing.
type PaymentDepositUseCase struct {
	txRepo    domain.TransactionRepository
	walletSvc domain.WalletService
	userSvc   domain.UserService
	provider  domain.PaymentProvider
}

// New creates a new PaymentDepositUseCase with the required dependencies.
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
func (uc *PaymentDepositUseCase) Execute(ctx context.Context, userID string, amount int64, cardToken string, idempotencyKey *string) (*domain.Transaction, error) {
	// Step 1: Check idempotency
	if existing, err := uc.checkIdempotency(ctx, userID, idempotencyKey); err != nil || existing != nil {
		return existing, err
	}

	// Step 2: Validate input
	if err := uc.validateInput(amount); err != nil {
		return nil, err
	}

	// Step 3: Verify user is active
	if err := uc.verifyUserActive(ctx, userID); err != nil {
		return nil, err
	}

	// Step 4: Get wallet
	_, walletID, err := uc.walletSvc.GetBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Step 5: Create pending transaction (Record First pattern)
	tx := uc.createPendingTransaction(userID, walletID, amount, idempotencyKey)
	if err := uc.txRepo.Create(ctx, tx); err != nil {
		return uc.handleDuplicateCreation(ctx, userID, idempotencyKey, err)
	}

	// Step 6: Process payment
	providerRef, err := uc.processPayment(ctx, tx.ID, amount, cardToken, userID, idempotencyKey)
	if err != nil {
		uc.markProviderFailed(ctx, tx.ID)
		return nil, err
	}

	// Step 7: Mark as charged
	tx.ProviderRef = &providerRef
	_ = uc.txRepo.UpdateStatus(ctx, tx.ID, domain.TxProviderCharged)

	// Step 8: Credit wallet
	if err := uc.walletSvc.Credit(ctx, userID, amount); err != nil {
		return uc.handleWalletFailure(ctx, tx)
	}

	// Step 9: Mark as completed
	if err := uc.txRepo.UpdateStatus(ctx, tx.ID, domain.TxCompleted); err != nil {
		// Already credited, status update failed - return success anyway
		tx.Status = domain.TxProviderCharged
		return tx, nil
	}

	tx.Status = domain.TxCompleted
	return tx, nil
}

// checkIdempotency checks for existing transaction with same idempotency key.
func (uc *PaymentDepositUseCase) checkIdempotency(ctx context.Context, userID string, idempotencyKey *string) (*domain.Transaction, error) {
	if idempotencyKey == nil || *idempotencyKey == "" {
		return nil, nil
	}

	existing, err := uc.txRepo.GetByUserAndIdempotencyKey(ctx, userID, *idempotencyKey)
	if err == nil {
		return existing, nil
	}

	if !errors.Is(err, apperrors.ErrNotFound) {
		return nil, err
	}

	return nil, nil
}

// validateInput validates the deposit amount.
func (uc *PaymentDepositUseCase) validateInput(amount int64) error {
	if amount <= 0 {
		return apperrors.ErrInvalidInput
	}
	return nil
}

// verifyUserActive checks if the user account is active.
func (uc *PaymentDepositUseCase) verifyUserActive(ctx context.Context, userID string) error {
	isActive, err := uc.userSvc.IsActive(ctx, userID)
	if err != nil {
		return err
	}
	if !isActive {
		return apperrors.ErrUserNotActive
	}
	return nil
}

// createPendingTransaction creates a new pending transaction entity.
func (uc *PaymentDepositUseCase) createPendingTransaction(userID, walletID string, amount int64, idempotencyKey *string) *domain.Transaction {
	return &domain.Transaction{
		ID:             xid.New().String(),
		UserID:         userID,
		WalletID:       walletID,
		Type:           domain.TxDeposit,
		Amount:         amount,
		Status:         domain.TxPending,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now(),
	}
}

// handleDuplicateCreation handles the case where transaction creation fails due to duplicate key.
func (uc *PaymentDepositUseCase) handleDuplicateCreation(ctx context.Context, userID string, idempotencyKey *string, err error) (*domain.Transaction, error) {
	if errors.Is(err, apperrors.ErrAlreadyExists) && idempotencyKey != nil {
		existing, _ := uc.txRepo.GetByUserAndIdempotencyKey(ctx, userID, *idempotencyKey)
		if existing != nil {
			return existing, nil
		}
	}
	return nil, err
}

// processPayment calls the payment provider and returns the provider reference or error.
func (uc *PaymentDepositUseCase) processPayment(ctx context.Context, txID string, amount int64, cardToken, userID string, idempotencyKey *string) (string, error) {
	providerKey := uc.generateProviderKey(userID, idempotencyKey, txID)

	resp, err := uc.provider.ProcessPayment(ctx, amount, cardToken, providerKey)
	if err != nil {
		return "", apperrors.ErrProviderError
	}

	if !resp.Success {
		return "", apperrors.NewProviderDeclined(resp.ErrorMessage)
	}

	return resp.Reference, nil
}

// generateProviderKey creates a hashed key for the payment provider.
func (uc *PaymentDepositUseCase) generateProviderKey(userID string, idempotencyKey *string, txID string) string {
	if idempotencyKey != nil && *idempotencyKey != "" {
		hash := sha256.Sum256([]byte(userID + *idempotencyKey))
		return fmt.Sprintf("%x", hash)[:32]
	}
	return txID
}

// markProviderFailed updates transaction status to provider_failed.
func (uc *PaymentDepositUseCase) markProviderFailed(ctx context.Context, txID string) {
	_ = uc.txRepo.UpdateStatus(ctx, txID, domain.TxProviderFailed)
}

// handleWalletFailure handles the case where wallet credit fails after provider charge.
func (uc *PaymentDepositUseCase) handleWalletFailure(ctx context.Context, tx *domain.Transaction) (*domain.Transaction, error) {
	_ = uc.txRepo.UpdateStatus(ctx, tx.ID, domain.TxWalletFailed)
	tx.Status = domain.TxWalletFailed
	return tx, apperrors.ErrWalletCreditFailed
}
