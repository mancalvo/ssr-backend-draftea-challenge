package refund

import (
	"context"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/domain"
	idgen "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/providers/idgen"
	timeprovider "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/providers/time"
)

// TransactionRunner executes operations within a database transaction.
type TransactionRunner interface {
	RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// UseCase defines the interface for refund operations.
type UseCase interface {
	Execute(ctx context.Context, userID, offeringID string, idempotencyKey *string) (*domain.Transaction, error)
}

// PaymentRefundUseCase orchestrates refund processing.
type PaymentRefundUseCase struct {
	txRunner   TransactionRunner
	txRepo     domain.TransactionRepository
	walletSvc  domain.WalletService
	entitleSvc domain.EntitlementService
	idGen      idgen.Generator
	timeProv   timeprovider.Provider
}

// New creates a new PaymentRefundUseCase with the required dependencies.
func New(
	txRunner TransactionRunner,
	txRepo domain.TransactionRepository,
	walletSvc domain.WalletService,
	entitleSvc domain.EntitlementService,
	idGen idgen.Generator,
	timeProv timeprovider.Provider,
) *PaymentRefundUseCase {
	return &PaymentRefundUseCase{
		txRunner:   txRunner,
		txRepo:     txRepo,
		walletSvc:  walletSvc,
		entitleSvc: entitleSvc,
		idGen:      idGen,
		timeProv:   timeProv,
	}
}

// Execute reverses a purchase: credits wallet and revokes entitlement.
// Uses transaction to ensure atomicity of credit + revoke operations.
func (uc *PaymentRefundUseCase) Execute(ctx context.Context, userID, offeringID string, idempotencyKey *string) (*domain.Transaction, error) {
	// Step 1: Get original purchase transaction
	originalTx, err := uc.getOriginalTransaction(ctx, userID, offeringID)
	if err != nil {
		return nil, err
	}

	// Step 2: Get wallet ID
	_, walletID, err := uc.walletSvc.GetBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Step 3: Prepare refund transaction
	refundTx := uc.prepareRefundTransaction(userID, walletID, offeringID, originalTx.Amount, idempotencyKey)

	// Step 4: Execute atomic refund operations
	if err := uc.executeRefundTransaction(ctx, userID, offeringID, originalTx.Amount, refundTx); err != nil {
		return nil, err
	}

	return refundTx, nil
}

// getOriginalTransaction retrieves the original purchase transaction for refund.
func (uc *PaymentRefundUseCase) getOriginalTransaction(ctx context.Context, userID, offeringID string) (*domain.Transaction, error) {
	// Get the original transaction ID via entitlement
	originalTxID, err := uc.entitleSvc.GetActiveEntitlementForOffering(ctx, userID, offeringID)
	if err != nil {
		return nil, err // No active entitlement found
	}

	// Get the original purchase transaction for the amount
	originalTx, err := uc.txRepo.GetByID(ctx, originalTxID)
	if err != nil {
		return nil, err
	}

	return originalTx, nil
}

// prepareRefundTransaction creates a new refund transaction entity.
func (uc *PaymentRefundUseCase) prepareRefundTransaction(userID, walletID, offeringID string, amount int64, idempotencyKey *string) *domain.Transaction {
	return &domain.Transaction{
		ID:             uc.idGen.Generate(),
		UserID:         userID,
		WalletID:       walletID,
		Type:           domain.TxRefund,
		Amount:         amount,
		Status:         domain.TxCompleted, // Will be committed as completed if transaction succeeds
		OfferingID:     &offeringID,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      uc.timeProv.Now(),
	}
}

// executeRefundTransaction executes the atomic refund operations within a transaction.
func (uc *PaymentRefundUseCase) executeRefundTransaction(ctx context.Context, userID, offeringID string, amount int64, refundTx *domain.Transaction) error {
	return uc.txRunner.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Create refund transaction record
		if err := uc.txRepo.Create(txCtx, refundTx); err != nil {
			return err
		}

		// Credit wallet
		if err := uc.walletSvc.Credit(txCtx, userID, amount); err != nil {
			return err
		}

		// Revoke entitlement
		if err := uc.entitleSvc.RevokeAccess(txCtx, userID, offeringID); err != nil {
			return err
		}

		return nil
	})
}
