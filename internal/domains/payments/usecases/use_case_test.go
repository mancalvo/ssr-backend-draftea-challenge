package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/domain"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/usecases"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
	"github.com/rs/xid"
)

// Mock TransactionRunner - executes function directly (no-op transaction for unit tests)
type mockTransactionRunner struct{}

func (m *mockTransactionRunner) RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// Mock Transaction Repository

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
	var txs []domain.Transaction
	for _, tx := range m.transactions {
		if tx.UserID == userID {
			txs = append(txs, *tx)
		}
	}
	return &domain.PaginatedTransactions{
		Transactions: txs,
		Page:         page,
		PageSize:     pageSize,
		TotalCount:   len(txs),
		TotalPages:   1,
	}, nil
}

// Mock Wallet Service

type mockWalletService struct {
	balances  map[string]int64  // userID -> balance
	walletIDs map[string]string // userID -> walletID
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
	if _, ok := m.walletIDs[userID]; !ok {
		return apperrors.ErrNotFound
	}
	if m.balances[userID] < amountCents {
		return apperrors.ErrInsufficientFunds
	}
	m.balances[userID] -= amountCents
	return nil
}

// Failing Wallet Service (for testing failure scenarios)

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
	if m.balances[userID] < amountCents {
		return apperrors.ErrInsufficientFunds
	}
	m.balances[userID] -= amountCents
	return nil
}

// Mock User Service

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

// Mock Offering Service

type mockOfferingService struct {
	prices    map[string]int64
	available map[string]bool
}

func newMockOfferingService() *mockOfferingService {
	return &mockOfferingService{
		prices:    make(map[string]int64),
		available: make(map[string]bool),
	}
}

func (m *mockOfferingService) GetPrice(ctx context.Context, offeringID string) (int64, error) {
	if price, ok := m.prices[offeringID]; ok {
		return price, nil
	}
	return 0, apperrors.ErrNotFound
}

func (m *mockOfferingService) IsAvailable(ctx context.Context, offeringID string) (bool, error) {
	if available, ok := m.available[offeringID]; ok {
		return available, nil
	}
	return false, apperrors.ErrNotFound
}

// Mock Entitlement Service

type mockEntitlementService struct {
	access map[string]bool   // "userID:offeringID" -> hasAccess
	txIDs  map[string]string // "userID:offeringID" -> transactionID
}

func newMockEntitlementService() *mockEntitlementService {
	return &mockEntitlementService{
		access: make(map[string]bool),
		txIDs:  make(map[string]string),
	}
}

func (m *mockEntitlementService) key(userID, offeringID string) string {
	return userID + ":" + offeringID
}

func (m *mockEntitlementService) GrantAccess(ctx context.Context, userID, offeringID, transactionID string) error {
	k := m.key(userID, offeringID)
	if m.access[k] {
		return apperrors.ErrAlreadyOwned
	}
	m.access[k] = true
	m.txIDs[k] = transactionID
	return nil
}

func (m *mockEntitlementService) RevokeAccess(ctx context.Context, userID, offeringID string) error {
	k := m.key(userID, offeringID)
	if !m.access[k] {
		return apperrors.ErrNotFound
	}
	m.access[k] = false
	return nil
}

func (m *mockEntitlementService) HasActiveAccess(ctx context.Context, userID, offeringID string) (bool, error) {
	return m.access[m.key(userID, offeringID)], nil
}

func (m *mockEntitlementService) GetActiveEntitlementForOffering(ctx context.Context, userID, offeringID string) (string, error) {
	k := m.key(userID, offeringID)
	if !m.access[k] {
		return "", apperrors.ErrNotFound
	}
	return m.txIDs[k], nil
}

// Failing Entitlement Service

type failingEntitlementService struct {
	access map[string]bool
	txIDs  map[string]string
}

func newFailingEntitlementService() *failingEntitlementService {
	return &failingEntitlementService{
		access: make(map[string]bool),
		txIDs:  make(map[string]string),
	}
}

func (m *failingEntitlementService) key(userID, offeringID string) string {
	return userID + ":" + offeringID
}

func (m *failingEntitlementService) GrantAccess(ctx context.Context, userID, offeringID, transactionID string) error {
	return nil
}

func (m *failingEntitlementService) RevokeAccess(ctx context.Context, userID, offeringID string) error {
	return errors.New("simulated revoke failure")
}

func (m *failingEntitlementService) HasActiveAccess(ctx context.Context, userID, offeringID string) (bool, error) {
	return m.access[m.key(userID, offeringID)], nil
}

func (m *failingEntitlementService) GetActiveEntitlementForOffering(ctx context.Context, userID, offeringID string) (string, error) {
	k := m.key(userID, offeringID)
	if txID, ok := m.txIDs[k]; ok {
		return txID, nil
	}
	return "", apperrors.ErrNotFound
}

// Mock Payment Provider

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

// Error Payment Provider (returns network/timeout errors)

type errorPaymentProvider struct{}

func (m *errorPaymentProvider) ProcessPayment(ctx context.Context, amount int64, cardToken string, idempotencyKey string) (*domain.ProviderResponse, error) {
	return nil, errors.New("network timeout")
}

// Tests

func TestDeposit_Success(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	tx, err := uc.Deposit(context.Background(), userID, 10000, "tok_test", nil)
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
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	userSvc.activeUsers[userID] = false

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	_, err := uc.Deposit(context.Background(), userID, 10000, "tok_test", nil)
	if err != apperrors.ErrUserNotActive {
		t.Errorf("expected ErrUserNotActive, got: %v", err)
	}
}

func TestDeposit_WalletCreditFailed(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newFailingWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	tx, err := uc.Deposit(context.Background(), userID, 10000, "tok_test", nil)

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
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	userSvc.activeUsers[userID] = true

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	// Test zero amount
	_, err := uc.Deposit(context.Background(), userID, 0, "tok_test", nil)
	if err != apperrors.ErrInvalidInput {
		t.Errorf("expected ErrInvalidInput for zero amount, got: %v", err)
	}

	// Test negative amount
	_, err = uc.Deposit(context.Background(), userID, -100, "tok_test", nil)
	if err != apperrors.ErrInvalidInput {
		t.Errorf("expected ErrInvalidInput for negative amount, got: %v", err)
	}
}

func TestDeposit_IdempotencyKey_ReturnsCached(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	idempotencyKey := "deposit-123"

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	// First deposit
	tx1, err := uc.Deposit(context.Background(), userID, 10000, "tok_test", &idempotencyKey)
	if err != nil {
		t.Fatalf("expected no error on first deposit, got: %v", err)
	}

	// Second deposit with same idempotency key should return cached result
	tx2, err := uc.Deposit(context.Background(), userID, 10000, "tok_test", &idempotencyKey)
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
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{shouldFail: true}

	userID := xid.New().String()
	walletID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	_, err := uc.Deposit(context.Background(), userID, 10000, "tok_test", nil)

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
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &errorPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	_, err := uc.Deposit(context.Background(), userID, 10000, "tok_test", nil)
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
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	// Note: NOT setting walletSvc.walletIDs[userID] - user has no wallet

	userSvc.activeUsers[userID] = true

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	_, err := uc.Deposit(context.Background(), userID, 10000, "tok_test", nil)
	if err != apperrors.ErrNotFound {
		t.Errorf("expected ErrNotFound for wallet not found, got: %v", err)
	}
}

func TestPurchase_Success(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 10000
	offeringSvc.prices[offeringID] = 5000
	offeringSvc.available[offeringID] = true

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	tx, err := uc.Purchase(context.Background(), userID, offeringID, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if tx.Amount != 5000 {
		t.Errorf("expected amount 5000, got %d", tx.Amount)
	}

	if tx.Type != domain.TxPurchase {
		t.Errorf("expected type purchase, got %s", tx.Type)
	}

	// Check wallet balance debited
	balance, _, _ := walletSvc.GetBalance(context.Background(), userID)
	if balance != 5000 {
		t.Errorf("expected wallet balance 5000, got %d", balance)
	}

	// Check entitlement granted
	hasAccess, _ := entitleSvc.HasActiveAccess(context.Background(), userID, offeringID)
	if !hasAccess {
		t.Error("expected user to have access to offering")
	}
}

func TestPurchase_InsufficientFunds(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 1000 // Only 1000, offering costs 5000
	offeringSvc.prices[offeringID] = 5000
	offeringSvc.available[offeringID] = true

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	_, err := uc.Purchase(context.Background(), userID, offeringID, nil)
	if err != apperrors.ErrInsufficientFunds {
		t.Errorf("expected ErrInsufficientFunds, got: %v", err)
	}
}

func TestPurchase_InactiveUser(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	offeringID := xid.New().String()

	userSvc.activeUsers[userID] = false // Inactive user

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	_, err := uc.Purchase(context.Background(), userID, offeringID, nil)
	if err != apperrors.ErrUserNotActive {
		t.Errorf("expected ErrUserNotActive, got: %v", err)
	}
}

func TestPurchase_OfferingNotFound(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 10000
	// Note: NOT setting offeringSvc.available[offeringID] - offering doesn't exist

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	_, err := uc.Purchase(context.Background(), userID, offeringID, nil)
	if err != apperrors.ErrNotFound {
		t.Errorf("expected ErrNotFound for non-existent offering, got: %v", err)
	}
}

func TestPurchase_OfferingNotAvailable(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 10000
	offeringSvc.available[offeringID] = false // Offering exists but not available

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	_, err := uc.Purchase(context.Background(), userID, offeringID, nil)
	if err != apperrors.ErrNotFound {
		t.Errorf("expected ErrNotFound for unavailable offering, got: %v", err)
	}
}

func TestPurchase_AlreadyOwned(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 10000
	offeringSvc.prices[offeringID] = 5000
	offeringSvc.available[offeringID] = true
	// User already has access
	entitleSvc.access[entitleSvc.key(userID, offeringID)] = true

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	_, err := uc.Purchase(context.Background(), userID, offeringID, nil)
	if err != apperrors.ErrAlreadyOwned {
		t.Errorf("expected ErrAlreadyOwned, got: %v", err)
	}
}

func TestPurchase_IdempotencyKey_ReturnsCached(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()
	idempotencyKey := "purchase-123"

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 10000
	offeringSvc.prices[offeringID] = 5000
	offeringSvc.available[offeringID] = true

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	// First purchase
	tx1, err := uc.Purchase(context.Background(), userID, offeringID, &idempotencyKey)
	if err != nil {
		t.Fatalf("expected no error on first purchase, got: %v", err)
	}

	// Second purchase with same idempotency key should return cached result
	tx2, err := uc.Purchase(context.Background(), userID, offeringID, &idempotencyKey)
	if err != nil {
		t.Fatalf("expected no error on idempotent purchase, got: %v", err)
	}

	if tx1.ID != tx2.ID {
		t.Errorf("expected same transaction ID for idempotent request, got %v and %v", tx1.ID, tx2.ID)
	}

	// Wallet should only be debited once
	balance, _, _ := walletSvc.GetBalance(context.Background(), userID)
	if balance != 5000 {
		t.Errorf("expected wallet balance 5000 (single debit), got %d", balance)
	}
}

func TestPurchase_GrantAccessRaceCondition(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 10000
	offeringSvc.prices[offeringID] = 5000
	offeringSvc.available[offeringID] = true

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	// First purchase succeeds
	_, err := uc.Purchase(context.Background(), userID, offeringID, nil)
	if err != nil {
		t.Fatalf("expected no error on first purchase, got: %v", err)
	}

	// Reset wallet balance for second attempt (simulate race where wallet debit succeeds)
	walletSvc.balances[userID] = 10000

	// Second purchase should fail because entitlement already exists
	_, err = uc.Purchase(context.Background(), userID, offeringID, nil)
	if err != apperrors.ErrAlreadyOwned {
		t.Errorf("expected ErrAlreadyOwned on race condition, got: %v", err)
	}
}

func TestRefund_Success(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()
	purchaseTxID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	// Simulate existing purchase
	txRepo.transactions[purchaseTxID] = &domain.Transaction{
		ID:         purchaseTxID,
		UserID:     userID,
		WalletID:   walletID,
		Type:       domain.TxPurchase,
		Amount:     5000,
		OfferingID: &offeringID,
	}
	entitleSvc.access[entitleSvc.key(userID, offeringID)] = true
	entitleSvc.txIDs[entitleSvc.key(userID, offeringID)] = purchaseTxID

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	refundTx, err := uc.Refund(context.Background(), userID, offeringID, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if refundTx.Type != domain.TxRefund {
		t.Errorf("expected type refund, got %s", refundTx.Type)
	}

	if refundTx.Status != domain.TxCompleted {
		t.Errorf("expected status completed, got %s", refundTx.Status)
	}

	// Check wallet balance credited
	balance, _, _ := walletSvc.GetBalance(context.Background(), userID)
	if balance != 5000 {
		t.Errorf("expected wallet balance 5000, got %d", balance)
	}

	// Check entitlement revoked
	hasAccess, _ := entitleSvc.HasActiveAccess(context.Background(), userID, offeringID)
	if hasAccess {
		t.Error("expected user to NOT have access after refund")
	}
}

func TestRefund_CreditFailed(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newFailingWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()
	purchaseTxID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	// Simulate existing purchase
	txRepo.transactions[purchaseTxID] = &domain.Transaction{
		ID:         purchaseTxID,
		UserID:     userID,
		WalletID:   walletID,
		Type:       domain.TxPurchase,
		Amount:     5000,
		OfferingID: &offeringID,
	}
	entitleSvc.access[entitleSvc.key(userID, offeringID)] = true
	entitleSvc.txIDs[entitleSvc.key(userID, offeringID)] = purchaseTxID

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	refundTx, err := uc.Refund(context.Background(), userID, offeringID, nil)

	// With atomic transactions, credit failure causes rollback - no transaction returned
	if err == nil {
		t.Error("expected error on credit failure")
	}

	if refundTx != nil {
		t.Errorf("expected nil transaction on failure (atomic rollback), got: %+v", refundTx)
	}
}

func TestRefund_RevokeFailed(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newFailingEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()
	purchaseTxID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	// Simulate existing purchase
	txRepo.transactions[purchaseTxID] = &domain.Transaction{
		ID:         purchaseTxID,
		UserID:     userID,
		WalletID:   walletID,
		Type:       domain.TxPurchase,
		Amount:     5000,
		OfferingID: &offeringID,
	}
	entitleSvc.access[entitleSvc.key(userID, offeringID)] = true
	entitleSvc.txIDs[entitleSvc.key(userID, offeringID)] = purchaseTxID

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	refundTx, err := uc.Refund(context.Background(), userID, offeringID, nil)

	// With atomic transactions, revoke failure causes rollback - no transaction returned
	if err == nil {
		t.Error("expected error on revoke failure")
	}

	if refundTx != nil {
		t.Errorf("expected nil transaction on failure (atomic rollback), got: %+v", refundTx)
	}

	// Note: In a real database transaction, the wallet credit would be rolled back.
	// The mock TransactionRunner can't simulate true rollback of in-memory changes,
	// so we don't check balance here. Integration tests should verify this behavior.
}

func TestRefund_NoActiveEntitlement(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0
	// Note: NOT setting entitleSvc.access - user doesn't own this offering

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	_, err := uc.Refund(context.Background(), userID, offeringID, nil)
	if err != apperrors.ErrNotFound {
		t.Errorf("expected ErrNotFound for no active entitlement, got: %v", err)
	}
}

func TestRefund_OriginalTxNotFound(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()
	purchaseTxID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	// Set up entitlement but NOT the transaction (data inconsistency scenario)
	entitleSvc.access[entitleSvc.key(userID, offeringID)] = true
	entitleSvc.txIDs[entitleSvc.key(userID, offeringID)] = purchaseTxID
	// Note: NOT adding txRepo.transactions[purchaseTxID]

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	_, err := uc.Refund(context.Background(), userID, offeringID, nil)
	if err != apperrors.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing original transaction, got: %v", err)
	}
}

func TestRefund_IdempotencyKey_ReturnsCached(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()
	provider := &mockPaymentProvider{}

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()
	purchaseTxID := xid.New().String()
	idempotencyKey := "refund-123"

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	// Simulate existing purchase
	txRepo.transactions[purchaseTxID] = &domain.Transaction{
		ID:         purchaseTxID,
		UserID:     userID,
		WalletID:   walletID,
		Type:       domain.TxPurchase,
		Amount:     5000,
		OfferingID: &offeringID,
	}
	entitleSvc.access[entitleSvc.key(userID, offeringID)] = true
	entitleSvc.txIDs[entitleSvc.key(userID, offeringID)] = purchaseTxID

	uc := usecases.NewPaymentUseCases(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, provider)

	// First refund
	tx1, err := uc.Refund(context.Background(), userID, offeringID, &idempotencyKey)
	if err != nil {
		t.Fatalf("expected no error on first refund, got: %v", err)
	}

	// Second refund with same idempotency key should return cached result
	tx2, err := uc.Refund(context.Background(), userID, offeringID, &idempotencyKey)
	if err != nil {
		t.Fatalf("expected no error on idempotent refund, got: %v", err)
	}

	if tx1.ID != tx2.ID {
		t.Errorf("expected same transaction ID for idempotent request, got %v and %v", tx1.ID, tx2.ID)
	}

	// Wallet should only be credited once
	balance, _, _ := walletSvc.GetBalance(context.Background(), userID)
	if balance != 5000 {
		t.Errorf("expected wallet balance 5000 (single credit), got %d", balance)
	}
}

// Unused but kept for reference
var _ = time.Now
