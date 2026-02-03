package purchase

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

type PaymentPurchaseUseCase struct {
	txRunner    uow.TransactionRunner
	txRepo      domain.TransactionRepository
	walletSvc   domain.WalletService
	userSvc     domain.UserService
	offeringSvc domain.OfferingService
	entitleSvc  domain.EntitlementService
}

func New(
	txRunner uow.TransactionRunner,
	txRepo domain.TransactionRepository,
	walletSvc domain.WalletService,
	userSvc domain.UserService,
	offeringSvc domain.OfferingService,
	entitleSvc domain.EntitlementService,
) *PaymentPurchaseUseCase {
	return &PaymentPurchaseUseCase{
		txRunner:    txRunner,
		txRepo:      txRepo,
		walletSvc:   walletSvc,
		userSvc:     userSvc,
		offeringSvc: offeringSvc,
		entitleSvc:  entitleSvc,
	}
}

// Execute debits wallet and grants access to an offering
// Uses transaction to ensure atomicity of debit + transaction record + entitlement grant
func (uc *PaymentPurchaseUseCase) Execute(ctx context.Context, userID, offeringID string, idempotencyKey *string) (*domain.Transaction, error) {
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

	// Validation checks - outside transaction for early exit
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

	// Check if user already owns this offering - outside transaction for early exit
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

	// Get wallet ID before transaction
	_, walletID, err := uc.walletSvc.GetBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Prepare transaction record
	txID := xid.New().String()
	tx := &domain.Transaction{
		ID:             txID,
		UserID:         userID,
		WalletID:       walletID,
		Type:           domain.TxPurchase,
		Amount:         price,
		Status:         domain.TxCompleted,
		OfferingID:     &offeringID,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now(),
	}

	// Critical operations wrapped in transaction for atomicity
	err = uc.txRunner.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Debit wallet (service handles insufficient funds check)
		if err := uc.walletSvc.Debit(txCtx, userID, price); err != nil {
			return err
		}

		// Create transaction record
		if err := uc.txRepo.Create(txCtx, tx); err != nil {
			if errors.Is(err, apperrors.ErrAlreadyExists) && idempotencyKey != nil {
				// Idempotency key collision - will be handled after transaction
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

	return tx, nil
}
