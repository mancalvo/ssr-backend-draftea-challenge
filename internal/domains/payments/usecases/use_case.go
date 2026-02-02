package usecases

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/domain"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
)

type PaymentUseCases interface {
	Deposit(ctx context.Context, userID uuid.UUID, amount int64, cardToken string, idempotencyKey *string) (*domain.Transaction, error)
	Purchase(ctx context.Context, userID, offeringID uuid.UUID, idempotencyKey *string) (*domain.Transaction, error)
	Refund(ctx context.Context, userID, offeringID uuid.UUID, idempotencyKey *string) (*domain.Transaction, error)
	GetHistory(ctx context.Context, userID uuid.UUID, page, pageSize int) (*domain.PaginatedTransactions, error)
}

type paymentUseCases struct {
	txRepo       domain.TransactionRepository
	walletSvc    domain.WalletService
	userSvc      domain.UserService
	offeringSvc  domain.OfferingService
	entitleSvc   domain.EntitlementService
	provider     domain.PaymentProvider
}

func NewPaymentUseCases(
	txRepo domain.TransactionRepository,
	walletSvc domain.WalletService,
	userSvc domain.UserService,
	offeringSvc domain.OfferingService,
	entitleSvc domain.EntitlementService,
	provider domain.PaymentProvider,
) PaymentUseCases {
	return &paymentUseCases{
		txRepo:      txRepo,
		walletSvc:   walletSvc,
		userSvc:     userSvc,
		offeringSvc: offeringSvc,
		entitleSvc:  entitleSvc,
		provider:    provider,
	}
}

// Deposit processes a card payment and credits the wallet.
// Uses "Record First" pattern: creates PENDING transaction before calling provider.
// This ensures we always have a record for reconciliation if provider call succeeds but DB fails.
// If idempotencyKey is provided, returns cached result for duplicate requests.
func (uc *paymentUseCases) Deposit(ctx context.Context, userID uuid.UUID, amount int64, cardToken string, idempotencyKey *string) (*domain.Transaction, error) {
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

	tx := &domain.Transaction{
		ID:             uuid.New(),
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
		hash := sha256.Sum256([]byte(userID.String() + *idempotencyKey))
		providerKey = fmt.Sprintf("%x", hash)[:32] // First 32 hex chars
	} else {
		// Fallback: use transaction ID if no idempotency key
		providerKey = tx.ID.String()
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

// Purchase debits wallet and grants access to an offering
func (uc *paymentUseCases) Purchase(ctx context.Context, userID, offeringID uuid.UUID, idempotencyKey *string) (*domain.Transaction, error) {
	// Check idempotency key first (scoped by user for security)
	if idempotencyKey != nil && *idempotencyKey != "" {
		existing, err := uc.txRepo.GetByUserAndIdempotencyKey(ctx, userID, *idempotencyKey)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, apperrors.ErrNotFound) {
			return nil, err
		}
	}

	isActive, err := uc.userSvc.IsActive(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !isActive {
		return nil, apperrors.ErrUserNotActive
	}

	isAvailable, err := uc.offeringSvc.IsAvailable(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	if !isAvailable {
		return nil, apperrors.ErrNotFound
	}

	// Check if user already owns this offering
	hasAccess, err := uc.entitleSvc.HasActiveAccess(ctx, userID, offeringID)
	if err != nil {
		return nil, err
	}
	if hasAccess {
		return nil, apperrors.ErrAlreadyOwned
	}

	price, err := uc.offeringSvc.GetPrice(ctx, offeringID)
	if err != nil {
		return nil, err
	}

	// Debit wallet (service handles insufficient funds check)
	if err := uc.walletSvc.Debit(ctx, userID, price); err != nil {
		return nil, err
	}

	_, walletID, err := uc.walletSvc.GetBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	tx := &domain.Transaction{
		ID:             uuid.New(),
		UserID:         userID,
		WalletID:       walletID,
		Type:           domain.TxPurchase,
		Amount:         price,
		Status:         domain.TxCompleted,
		OfferingID:     &offeringID,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now(),
	}

	if err := uc.txRepo.Create(ctx, tx); err != nil {
		if errors.Is(err, apperrors.ErrAlreadyExists) && idempotencyKey != nil {
			existing, _ := uc.txRepo.GetByUserAndIdempotencyKey(ctx, userID, *idempotencyKey)
			if existing != nil {
				return existing, nil
			}
		}
		// Transaction record failed but wallet debited - needs reconciliation
		return nil, err
	}

	// Grant entitlement via service
	if err := uc.entitleSvc.GrantAccess(ctx, userID, offeringID, tx.ID); err != nil {
		// Repository returns ErrAlreadyOwned for duplicate (race condition)
		if errors.Is(err, apperrors.ErrAlreadyOwned) {
			return nil, apperrors.ErrAlreadyOwned
		}
		// Entitlement failed but wallet debited - needs reconciliation
		return nil, err
	}

	return tx, nil
}

// Refund reverses a purchase: credits wallet and revokes entitlement
// Uses granular status tracking for eventual consistency and reconciliation
func (uc *paymentUseCases) Refund(ctx context.Context, userID, offeringID uuid.UUID, idempotencyKey *string) (*domain.Transaction, error) {
	// Check idempotency key first (scoped by user for security)
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

	// Create refund transaction (PENDING)
	refundTx := &domain.Transaction{
		ID:             uuid.New(),
		UserID:         userID,
		WalletID:       walletID,
		Type:           domain.TxRefund,
		Amount:         originalTx.Amount,
		Status:         domain.TxPending,
		OfferingID:     &offeringID,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now(),
	}

	if err := uc.txRepo.Create(ctx, refundTx); err != nil {
		if errors.Is(err, apperrors.ErrAlreadyExists) && idempotencyKey != nil {
			existing, _ := uc.txRepo.GetByUserAndIdempotencyKey(ctx, userID, *idempotencyKey)
			if existing != nil {
				return existing, nil
			}
		}
		return nil, err
	}

	// Step 1: Credit wallet via service
	creditErr := uc.walletSvc.Credit(ctx, userID, originalTx.Amount)
	if creditErr != nil {
		_ = uc.txRepo.UpdateStatus(ctx, refundTx.ID, domain.TxCreditFailed)
		refundTx.Status = domain.TxCreditFailed
		return refundTx, apperrors.ErrWalletCreditFailed
	}
	_ = uc.txRepo.UpdateStatus(ctx, refundTx.ID, domain.TxWalletCredited)

	// Step 2: Revoke entitlement via service
	revokeErr := uc.entitleSvc.RevokeAccess(ctx, userID, offeringID)
	if revokeErr != nil {
		_ = uc.txRepo.UpdateStatus(ctx, refundTx.ID, domain.TxRevokeFailed)
		refundTx.Status = domain.TxRevokeFailed
		return refundTx, apperrors.ErrRevokeFailed
	}

	// Both succeeded - mark as completed
	_ = uc.txRepo.UpdateStatus(ctx, refundTx.ID, domain.TxCompleted)
	refundTx.Status = domain.TxCompleted

	return refundTx, nil
}

// GetHistory returns paginated transaction history
func (uc *paymentUseCases) GetHistory(ctx context.Context, userID uuid.UUID, page, pageSize int) (*domain.PaginatedTransactions, error) {
	isActive, err := uc.userSvc.IsActive(ctx, userID)
	if err != nil {
		return nil, err
	}
	_ = isActive // Just verifying user exists

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	return uc.txRepo.GetByUserID(ctx, userID, page, pageSize)
}

