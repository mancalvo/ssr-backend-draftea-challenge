package purchase

import (
	"context"
	"errors"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/domain"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
	idgen "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/providers/idgen"
	timeprovider "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/providers/time"
)

// TransactionRunner executes operations within a database transaction.
type TransactionRunner interface {
	RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// UseCase defines the interface for purchase operations.
type UseCase interface {
	Execute(ctx context.Context, userID, offeringID string, idempotencyKey *string) (*domain.Transaction, error)
}

// PaymentPurchaseUseCase orchestrates purchase processing.
type PaymentPurchaseUseCase struct {
	txRunner    TransactionRunner
	txRepo      domain.TransactionRepository
	walletSvc   domain.WalletService
	userSvc     domain.UserService
	offeringSvc domain.OfferingService
	entitleSvc  domain.EntitlementService
	idGen       idgen.Generator
	timeProv    timeprovider.Provider
}

// New creates a new PaymentPurchaseUseCase with the required dependencies.
func New(
	txRunner TransactionRunner,
	txRepo domain.TransactionRepository,
	walletSvc domain.WalletService,
	userSvc domain.UserService,
	offeringSvc domain.OfferingService,
	entitleSvc domain.EntitlementService,
	idGen idgen.Generator,
	timeProv timeprovider.Provider,
) *PaymentPurchaseUseCase {
	return &PaymentPurchaseUseCase{
		txRunner:    txRunner,
		txRepo:      txRepo,
		walletSvc:   walletSvc,
		userSvc:     userSvc,
		offeringSvc: offeringSvc,
		entitleSvc:  entitleSvc,
		idGen:       idGen,
		timeProv:    timeProv,
	}
}

// Execute debits wallet and grants access to an offering.
// Uses transaction to ensure atomicity of debit + transaction record + entitlement grant.
func (uc *PaymentPurchaseUseCase) Execute(ctx context.Context, userID, offeringID string, idempotencyKey *string) (*domain.Transaction, error) {
	// Step 1: Check idempotency (outside transaction for early exit)
	if existing, err := uc.checkIdempotency(ctx, userID, idempotencyKey); err != nil || existing != nil {
		return existing, err
	}

	// Step 2: Validate user is active
	if err := uc.validateUser(ctx, userID); err != nil {
		return nil, err
	}

	// Step 3: Validate offering is available
	if err := uc.validateOffering(ctx, offeringID); err != nil {
		return nil, err
	}

	// Step 4: Check user doesn't already own this offering
	if err := uc.checkNotAlreadyOwned(ctx, userID, offeringID); err != nil {
		return nil, err
	}

	// Step 5: Get offering price
	price, err := uc.getOfferingPrice(ctx, offeringID)
	if err != nil {
		return nil, err
	}

	// Step 6: Get wallet ID
	_, walletID, err := uc.walletSvc.GetBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Step 7: Prepare transaction record
	tx := uc.prepareTransaction(userID, walletID, offeringID, price, idempotencyKey)

	// Step 8: Execute atomic operations within transaction
	if err := uc.executePurchaseTransaction(ctx, userID, offeringID, price, tx, idempotencyKey); err != nil {
		return uc.handleTransactionError(ctx, userID, idempotencyKey, err)
	}

	return tx, nil
}

// checkIdempotency checks for existing transaction with same idempotency key.
func (uc *PaymentPurchaseUseCase) checkIdempotency(ctx context.Context, userID string, idempotencyKey *string) (*domain.Transaction, error) {
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

// validateUser checks if the user account is active.
func (uc *PaymentPurchaseUseCase) validateUser(ctx context.Context, userID string) error {
	isActive, err := uc.userSvc.IsActive(ctx, userID)
	if err != nil {
		return err
	}
	if !isActive {
		return apperrors.ErrUserNotActive
	}
	return nil
}

// validateOffering checks if the offering is available.
func (uc *PaymentPurchaseUseCase) validateOffering(ctx context.Context, offeringID string) error {
	isAvailable, err := uc.offeringSvc.IsAvailable(ctx, offeringID)
	if err != nil {
		return err
	}
	if !isAvailable {
		return apperrors.ErrNotFound
	}
	return nil
}

// checkNotAlreadyOwned verifies the user doesn't already own this offering.
func (uc *PaymentPurchaseUseCase) checkNotAlreadyOwned(ctx context.Context, userID, offeringID string) error {
	hasAccess, err := uc.entitleSvc.HasActiveAccess(ctx, userID, offeringID)
	if err != nil {
		return err
	}
	if hasAccess {
		return apperrors.ErrAlreadyOwned
	}
	return nil
}

// getOfferingPrice retrieves the price for an offering.
func (uc *PaymentPurchaseUseCase) getOfferingPrice(ctx context.Context, offeringID string) (int64, error) {
	return uc.offeringSvc.GetPrice(ctx, offeringID)
}

// prepareTransaction creates a new transaction entity for the purchase.
func (uc *PaymentPurchaseUseCase) prepareTransaction(userID, walletID, offeringID string, price int64, idempotencyKey *string) *domain.Transaction {
	return &domain.Transaction{
		ID:             uc.idGen.Generate(),
		UserID:         userID,
		WalletID:       walletID,
		Type:           domain.TxPurchase,
		Amount:         price,
		Status:         domain.TxCompleted,
		OfferingID:     &offeringID,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      uc.timeProv.Now(),
	}
}

// executePurchaseTransaction executes the atomic purchase operations within a transaction.
func (uc *PaymentPurchaseUseCase) executePurchaseTransaction(ctx context.Context, userID, offeringID string, price int64, tx *domain.Transaction, idempotencyKey *string) error {
	return uc.txRunner.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Debit wallet
		if err := uc.walletSvc.Debit(txCtx, userID, price); err != nil {
			return err
		}

		// Create transaction record
		if err := uc.txRepo.Create(txCtx, tx); err != nil {
			if errors.Is(err, apperrors.ErrAlreadyExists) && idempotencyKey != nil {
				return err
			}
			return err
		}

		// Grant entitlement
		if err := uc.entitleSvc.GrantAccess(txCtx, userID, offeringID, tx.ID); err != nil {
			return err
		}

		return nil
	})
}

// handleTransactionError handles errors from the purchase transaction.
func (uc *PaymentPurchaseUseCase) handleTransactionError(ctx context.Context, userID string, idempotencyKey *string, err error) (*domain.Transaction, error) {
	// Check for idempotency collision
	if errors.Is(err, apperrors.ErrAlreadyExists) && idempotencyKey != nil {
		existing, _ := uc.txRepo.GetByUserAndIdempotencyKey(ctx, userID, *idempotencyKey)
		if existing != nil {
			return existing, nil
		}
	}
	return nil, err
}
