package history

import (
	"context"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/domain"
)

// TransactionReader retrieves transaction history.
type TransactionReader interface {
	GetByUserID(ctx context.Context, userID string, page, pageSize int) (*domain.PaginatedTransactions, error)
}

// UserService verifies user exists.
type UserService interface {
	IsActive(ctx context.Context, userID string) (bool, error)
}

type UseCase interface {
	Execute(ctx context.Context, userID string, page, pageSize int) (*domain.PaginatedTransactions, error)
}

type PaymentHistoryUseCase struct {
	txRepo  TransactionReader
	userSvc UserService
}

func New(
	txRepo TransactionReader,
	userSvc UserService,
) *PaymentHistoryUseCase {
	return &PaymentHistoryUseCase{
		txRepo:  txRepo,
		userSvc: userSvc,
	}
}

// Execute returns paginated transaction history
func (uc *PaymentHistoryUseCase) Execute(ctx context.Context, userID string, page, pageSize int) (*domain.PaginatedTransactions, error) {
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
