package deposit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/domain"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/usecases/deposit"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
	"github.com/rs/xid"
)

// Mocks

type mockTransactionRepo struct {
	transactions map[string]*domain.Transaction
}

func newMockTransactionRepo() *mockTransactionRepo {
	return &mockTransactionRepo{transactions: make(map[string]*domain.Transaction)}
}

func (m *mockTransactionRepo) Create(ctx context.Context, tx *domain.Transaction) error {
	m.transactions[tx.ID] = tx
	return nil
}

func (m *mockTransactionRepo) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	if tx, ok := m.transactions[id]; ok {
		return tx, nil
	}
	return nil, apperrors.ErrNotFound
}

func (m *mockTransactionRepo) UpdateStatus(ctx context.Context, id string, status domain.TransactionStatus) error {
	if tx, ok := m.transactions[id]; ok {
		tx.Status = status
		return nil
	}
	return apperrors.ErrNotFound
}

func (m *mockTransactionRepo) GetByUserAndIdempotencyKey(ctx context.Context, userID string, key string) (*domain.Transaction, error) {
	for _, tx := range m.transactions {
		if tx.UserID == userID && tx.IdempotencyKey != nil && *tx.IdempotencyKey == key {
			return tx, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *mockTransactionRepo) GetByUserID(ctx context.Context, userID string, page, pageSize int) (*domain.PaginatedTransactions, error) {
	return nil, nil // Not needed for deposit
}

type mockWalletService struct {
	balances  map[string]int64
	walletIDs map[string]string
}

func newMockWalletService() *mockWalletService {
	return &mockWalletService{
		balances:  make(map[string]int64),
		walletIDs: make(map[string]string),
	}
}

func (m *mockWalletService) GetBalance(ctx context.Context, userID string) (int64, string, error) {
	if walletID, ok := m.walletIDs[userID]; ok {
		return m.balances[userID], walletID, nil
	}
	return 0, "", apperrors.ErrNotFound
}

func (m *mockWalletService) Credit(ctx context.Context, userID string, amountCents int64) error {
	if _, ok := m.walletIDs[userID]; !ok {
		return apperrors.ErrNotFound
	}
	m.balances[userID] += amountCents
	return nil
}

func (m *mockWalletService) Debit(ctx context.Context, userID string, amountCents int64) error {
	return nil // Not needed for deposit
}

type failingWalletService struct {
	balances  map[string]int64
	walletIDs map[string]string
}

func newFailingWalletService() *failingWalletService {
	return &failingWalletService{
		balances:  make(map[string]int64),
		walletIDs: make(map[string]string),
	}
}

func (m *failingWalletService) GetBalance(ctx context.Context, userID string) (int64, string, error) {
	if walletID, ok := m.walletIDs[userID]; ok {
		return m.balances[userID], walletID, nil
	}
	return 0, "", apperrors.ErrNotFound
}

func (m *failingWalletService) Credit(ctx context.Context, userID string, amountCents int64) error {
	return errors.New("simulated credit failure")
}

func (m *failingWalletService) Debit(ctx context.Context, userID string, amountCents int64) error {
	return nil
}

type mockUserService struct {
	activeUsers map[string]bool
}

func newMockUserService() *mockUserService {
	return &mockUserService{activeUsers: make(map[string]bool)}
}

func (m *mockUserService) IsActive(ctx context.Context, userID string) (bool, error) {
	if active, ok := m.activeUsers[userID]; ok {
		return active, nil
	}
	return false, apperrors.ErrNotFound
}

type mockPaymentProvider struct {
	shouldFail bool
}

func (m *mockPaymentProvider) ProcessPayment(ctx context.Context, amount int64, cardToken string, idempotencyKey string) (*domain.ProviderResponse, error) {
	if m.shouldFail {
		return &domain.ProviderResponse{
			Success:      false,
			ErrorMessage: "payment declined",
		}, nil
	}
	return &domain.ProviderResponse{
		Success:   true,
		Reference: "test_ref_123",
	}, nil
}

type errorPaymentProvider struct{}

func (m *errorPaymentProvider) ProcessPayment(ctx context.Context, amount int64, cardToken string, idempotencyKey string) (*domain.ProviderResponse, error) {
	return nil, errors.New("network timeout")
}

// Tests

func TestDeposit_Success(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	uc := deposit.New(txRepo, walletSvc, userSvc, provider)

	tx, err := uc.Execute(context.Background(), userID, 10000, "tok_test", nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if tx.Amount != 10000 {
		t.Errorf("expected amount 10000, got %d", tx.Amount)
	}

	if tx.Type != domain.TxDeposit {
		t.Errorf("expected type deposit, got %s", tx.Type)
	}

	// Check wallet balance updated
	balance, _, _ := walletSvc.GetBalance(context.Background(), userID)
	if balance != 10000 {
		t.Errorf("expected wallet balance 10000, got %d", balance)
	}
}

func TestDeposit_InactiveUser(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	userSvc.activeUsers[userID] = false

	uc := deposit.New(txRepo, walletSvc, userSvc, provider)

	_, err := uc.Execute(context.Background(), userID, 10000, "tok_test", nil)
	if err != apperrors.ErrUserNotActive {
		t.Errorf("expected ErrUserNotActive, got: %v", err)
	}
}

func TestDeposit_WalletCreditFailed(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newFailingWalletService()
	userSvc := newMockUserService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	uc := deposit.New(txRepo, walletSvc, userSvc, provider)

	tx, err := uc.Execute(context.Background(), userID, 10000, "tok_test", nil)

	if err != apperrors.ErrWalletCreditFailed {
		t.Errorf("expected ErrWalletCreditFailed, got: %v", err)
	}

	if tx == nil {
		t.Fatal("expected transaction to be returned on wallet credit failure")
	}

	if tx.Status != domain.TxWalletFailed {
		t.Errorf("expected status wallet_failed, got %s", tx.Status)
	}
}

func TestDeposit_InvalidAmount(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	userSvc.activeUsers[userID] = true

	uc := deposit.New(txRepo, walletSvc, userSvc, provider)

	// Test zero amount
	_, err := uc.Execute(context.Background(), userID, 0, "tok_test", nil)
	if err != apperrors.ErrInvalidInput {
		t.Errorf("expected ErrInvalidInput for zero amount, got: %v", err)
	}

	// Test negative amount
	_, err = uc.Execute(context.Background(), userID, -100, "tok_test", nil)
	if err != apperrors.ErrInvalidInput {
		t.Errorf("expected ErrInvalidInput for negative amount, got: %v", err)
	}
}

func TestDeposit_IdempotencyKey_ReturnsCached(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	idempotencyKey := "deposit-123"

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	uc := deposit.New(txRepo, walletSvc, userSvc, provider)

	// First deposit
	tx1, err := uc.Execute(context.Background(), userID, 10000, "tok_test", &idempotencyKey)
	if err != nil {
		t.Fatalf("expected no error on first deposit, got: %v", err)
	}

	// Second deposit with same idempotency key should return cached result
	tx2, err := uc.Execute(context.Background(), userID, 10000, "tok_test", &idempotencyKey)
	if err != nil {
		t.Fatalf("expected no error on idempotent deposit, got: %v", err)
	}

	if tx1.ID != tx2.ID {
		t.Errorf("expected same transaction ID for idempotent request, got %v and %v", tx1.ID, tx2.ID)
	}

	// Wallet should only be credited once
	balance, _, _ := walletSvc.GetBalance(context.Background(), userID)
	if balance != 10000 {
		t.Errorf("expected wallet balance 10000 (single credit), got %d", balance)
	}
}

func TestDeposit_ProviderDeclined(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	provider := &mockPaymentProvider{shouldFail: true}

	userID := xid.New().String()
	walletID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	uc := deposit.New(txRepo, walletSvc, userSvc, provider)

	_, err := uc.Execute(context.Background(), userID, 10000, "tok_test", nil)

	// Should return a provider declined error
	var declinedErr *apperrors.ProviderDeclined
	if !errors.As(err, &declinedErr) {
		t.Errorf("expected ProviderDeclinedError, got: %v", err)
	}

	// Wallet should NOT be credited
	balance, _, _ := walletSvc.GetBalance(context.Background(), userID)
	if balance != 0 {
		t.Errorf("expected wallet balance 0 (no credit on declined), got %d", balance)
	}
}

func TestDeposit_ProviderError(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	provider := &errorPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	uc := deposit.New(txRepo, walletSvc, userSvc, provider)

	_, err := uc.Execute(context.Background(), userID, 10000, "tok_test", nil)
	if err != apperrors.ErrProviderError {
		t.Errorf("expected ErrProviderError, got: %v", err)
	}

	// Wallet should NOT be credited
	balance, _, _ := walletSvc.GetBalance(context.Background(), userID)
	if balance != 0 {
		t.Errorf("expected wallet balance 0 (no credit on provider error), got %d", balance)
	}
}

func TestDeposit_WalletNotFound(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()

	userSvc.activeUsers[userID] = true
	// Don't set walletID - simulates wallet not found

	uc := deposit.New(txRepo, walletSvc, userSvc, provider)

	_, err := uc.Execute(context.Background(), userID, 10000, "tok_test", nil)
	if !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// mockRaceConditionRepo simulates a race condition where two concurrent requests
// try to create a transaction with the same idempotency key simultaneously
type mockRaceConditionRepo struct {
	transactions     map[string]*domain.Transaction
	idempotencyKey   string
	existingTx       *domain.Transaction
	idempotencyCalls int // Track number of calls to GetByUserAndIdempotencyKey
}

func newMockRaceConditionRepo(idempotencyKey string, existingTx *domain.Transaction) *mockRaceConditionRepo {
	return &mockRaceConditionRepo{
		transactions:     make(map[string]*domain.Transaction),
		idempotencyKey:   idempotencyKey,
		existingTx:       existingTx,
		idempotencyCalls: 0,
	}
}

func (m *mockRaceConditionRepo) Create(ctx context.Context, tx *domain.Transaction) error {
	// Simulate race condition: return ErrAlreadyExists when idempotency key matches
	if tx.IdempotencyKey != nil && *tx.IdempotencyKey == m.idempotencyKey {
		return apperrors.ErrAlreadyExists
	}
	m.transactions[tx.ID] = tx
	return nil
}

func (m *mockRaceConditionRepo) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	if tx, ok := m.transactions[id]; ok {
		return tx, nil
	}
	return nil, apperrors.ErrNotFound
}

func (m *mockRaceConditionRepo) UpdateStatus(ctx context.Context, id string, status domain.TransactionStatus) error {
	if tx, ok := m.transactions[id]; ok {
		tx.Status = status
		return nil
	}
	return apperrors.ErrNotFound
}

func (m *mockRaceConditionRepo) GetByUserAndIdempotencyKey(ctx context.Context, userID string, key string) (*domain.Transaction, error) {
	// Simulate race condition:
	// First call (initial check) returns not found
	// Second call (after Create fails with ErrAlreadyExists) returns the existing transaction
	m.idempotencyCalls++

	if key == m.idempotencyKey && m.existingTx != nil {
		// Second call - return the transaction that was just created by another request
		if m.idempotencyCalls > 1 {
			return m.existingTx, nil
		}
		// First call - race condition, transaction not visible yet
		return nil, apperrors.ErrNotFound
	}
	return nil, apperrors.ErrNotFound
}

func (m *mockRaceConditionRepo) GetByUserID(ctx context.Context, userID string, page, pageSize int) (*domain.PaginatedTransactions, error) {
	return nil, nil
}

func TestDeposit_RaceCondition_IdempotencyKey(t *testing.T) {
	// Setup: Create an existing transaction that another concurrent request already created
	userID := xid.New().String()
	walletID := xid.New().String()
	idempotencyKey := "race-condition-key-123"

	existingTx := &domain.Transaction{
		ID:             xid.New().String(),
		UserID:         userID,
		WalletID:       walletID,
		Type:           domain.TxDeposit,
		Amount:         10000,
		Status:         domain.TxCompleted,
		IdempotencyKey: &idempotencyKey,
		ProviderRef:    strPtr("ref_existing"),
		CreatedAt:      time.Now(),
	}

	// Create repo that simulates race condition
	txRepo := newMockRaceConditionRepo(idempotencyKey, existingTx)

	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	provider := &mockPaymentProvider{}

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 50000 // Already has balance from the existing deposit

	uc := deposit.New(txRepo, walletSvc, userSvc, provider)

	// Execute - should handle the race condition gracefully
	tx, err := uc.Execute(context.Background(), userID, 10000, "tok_test", &idempotencyKey)

	if err != nil {
		t.Fatalf("expected no error when handling race condition, got: %v", err)
	}

	// Should return the existing transaction, not create a new one
	if tx.ID != existingTx.ID {
		t.Errorf("expected existing transaction ID %v, got %v", existingTx.ID, tx.ID)
	}

	// Wallet balance should remain unchanged (no double credit)
	balance, _, _ := walletSvc.GetBalance(context.Background(), userID)
	if balance != 50000 {
		t.Errorf("expected wallet balance 50000 (no double credit), got %d", balance)
	}
}

// Helper function
func strPtr(s string) *string {
	return &s
}
