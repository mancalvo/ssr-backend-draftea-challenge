package purchase_test

import (
	"context"
	"testing"
	"time"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/domain"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/usecases/purchase"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
	"github.com/rs/xid"
)

// Mocks

type mockTransactionRunner struct{}

type mockIDGenerator struct{}

func (g *mockIDGenerator) Generate() string {
	return xid.New().String()
}

type mockTimeProvider struct{}

func (p *mockTimeProvider) Now() time.Time {
	return time.Now()
}

func (m *mockTransactionRunner) RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

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
	return nil, nil // Not needed
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
	if _, ok := m.walletIDs[userID]; !ok {
		return apperrors.ErrNotFound
	}
	if m.balances[userID] < amountCents {
		return apperrors.ErrInsufficientFunds
	}
	m.balances[userID] -= amountCents
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

// Tests

func TestPurchase_Success(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	userSvc := newMockUserService()
	offeringSvc := newMockOfferingService()
	entitleSvc := newMockEntitlementService()

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 10000
	offeringSvc.prices[offeringID] = 5000
	offeringSvc.available[offeringID] = true

	uc := purchase.New(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, &mockIDGenerator{}, &mockTimeProvider{})

	tx, err := uc.Execute(context.Background(), userID, offeringID, nil)
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

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 1000 // Only 1000, offering costs 5000
	offeringSvc.prices[offeringID] = 5000
	offeringSvc.available[offeringID] = true

	uc := purchase.New(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, &mockIDGenerator{}, &mockTimeProvider{})

	_, err := uc.Execute(context.Background(), userID, offeringID, nil)
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

	userID := xid.New().String()
	offeringID := xid.New().String()

	userSvc.activeUsers[userID] = false // Inactive user

	uc := purchase.New(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, &mockIDGenerator{}, &mockTimeProvider{})

	_, err := uc.Execute(context.Background(), userID, offeringID, nil)
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

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 10000
	// Note: NOT setting offeringSvc.available[offeringID] - offering doesn't exist

	uc := purchase.New(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, &mockIDGenerator{}, &mockTimeProvider{})

	_, err := uc.Execute(context.Background(), userID, offeringID, nil)
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

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 10000
	offeringSvc.available[offeringID] = false // Offering exists but not available

	uc := purchase.New(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, &mockIDGenerator{}, &mockTimeProvider{})

	_, err := uc.Execute(context.Background(), userID, offeringID, nil)
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

	uc := purchase.New(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, &mockIDGenerator{}, &mockTimeProvider{})

	_, err := uc.Execute(context.Background(), userID, offeringID, nil)
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

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()
	idempotencyKey := "purchase-123"

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 10000
	offeringSvc.prices[offeringID] = 5000
	offeringSvc.available[offeringID] = true

	uc := purchase.New(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, &mockIDGenerator{}, &mockTimeProvider{})

	// First purchase
	tx1, err := uc.Execute(context.Background(), userID, offeringID, &idempotencyKey)
	if err != nil {
		t.Fatalf("expected no error on first purchase, got: %v", err)
	}

	// Second purchase with same idempotency key should return cached result
	tx2, err := uc.Execute(context.Background(), userID, offeringID, &idempotencyKey)
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

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()

	userSvc.activeUsers[userID] = true
	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 10000
	offeringSvc.prices[offeringID] = 5000
	offeringSvc.available[offeringID] = true

	uc := purchase.New(&mockTransactionRunner{}, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc, &mockIDGenerator{}, &mockTimeProvider{})

	// First purchase succeeds
	_, err := uc.Execute(context.Background(), userID, offeringID, nil)
	if err != nil {
		t.Fatalf("expected no error on first purchase, got: %v", err)
	}

	// Reset wallet balance for second attempt (simulate race where wallet debit succeeds)
	walletSvc.balances[userID] = 10000

	// Second purchase should fail because entitlement already exists
	_, err = uc.Execute(context.Background(), userID, offeringID, nil)
	if err != apperrors.ErrAlreadyOwned {
		t.Errorf("expected ErrAlreadyOwned on race condition, got: %v", err)
	}
}

// Unused but kept for reference
var _ = time.Now
