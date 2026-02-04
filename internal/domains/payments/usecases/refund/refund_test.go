package refund_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/domain"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/usecases/refund"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
	"github.com/rs/xid"
)

// Mocks

type mockIDGenerator struct{}

func (g *mockIDGenerator) Generate() string {
	return xid.New().String()
}

type mockTimeProvider struct{}

func (p *mockTimeProvider) Now() time.Time {
	return time.Now()
}

type mockTransactionRunner struct{}

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
	return nil // Not needed
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
	return nil // Not needed
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

// Tests

func TestRefund_Success(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	entitleSvc := newMockEntitlementService()

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()
	purchaseTxID := xid.New().String()

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

	uc := refund.New(&mockTransactionRunner{}, txRepo, walletSvc, entitleSvc, &mockIDGenerator{}, &mockTimeProvider{})

	refundTx, err := uc.Execute(context.Background(), userID, offeringID, nil)
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
	entitleSvc := newMockEntitlementService()

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()
	purchaseTxID := xid.New().String()

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

	uc := refund.New(&mockTransactionRunner{}, txRepo, walletSvc, entitleSvc, &mockIDGenerator{}, &mockTimeProvider{})

	refundTx, err := uc.Execute(context.Background(), userID, offeringID, nil)

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
	entitleSvc := newFailingEntitlementService()

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()
	purchaseTxID := xid.New().String()

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

	uc := refund.New(&mockTransactionRunner{}, txRepo, walletSvc, entitleSvc, &mockIDGenerator{}, &mockTimeProvider{})

	refundTx, err := uc.Execute(context.Background(), userID, offeringID, nil)

	// With atomic transactions, revoke failure causes rollback - no transaction returned
	if err == nil {
		t.Error("expected error on revoke failure")
	}

	if refundTx != nil {
		t.Errorf("expected nil transaction on failure (atomic rollback), got: %+v", refundTx)
	}
}

func TestRefund_NoActiveEntitlement(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	entitleSvc := newMockEntitlementService()

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()

	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0
	// Note: NOT setting entitleSvc.access - user doesn't own this offering

	uc := refund.New(&mockTransactionRunner{}, txRepo, walletSvc, entitleSvc, &mockIDGenerator{}, &mockTimeProvider{})

	_, err := uc.Execute(context.Background(), userID, offeringID, nil)
	if err != apperrors.ErrNotFound {
		t.Errorf("expected ErrNotFound for no active entitlement, got: %v", err)
	}
}

func TestRefund_OriginalTxNotFound(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	entitleSvc := newMockEntitlementService()

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()
	purchaseTxID := xid.New().String()

	walletSvc.walletIDs[userID] = walletID
	walletSvc.balances[userID] = 0

	// Set up entitlement but NOT the transaction (data inconsistency scenario)
	entitleSvc.access[entitleSvc.key(userID, offeringID)] = true
	entitleSvc.txIDs[entitleSvc.key(userID, offeringID)] = purchaseTxID
	// Note: NOT adding txRepo.transactions[purchaseTxID]

	uc := refund.New(&mockTransactionRunner{}, txRepo, walletSvc, entitleSvc, &mockIDGenerator{}, &mockTimeProvider{})

	_, err := uc.Execute(context.Background(), userID, offeringID, nil)
	if err != apperrors.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing original transaction, got: %v", err)
	}
}

func TestRefund_IdempotencyKey_ReturnsCached(t *testing.T) {
	txRepo := newMockTransactionRepo()
	walletSvc := newMockWalletService()
	entitleSvc := newMockEntitlementService()

	userID := xid.New().String()
	walletID := xid.New().String()
	offeringID := xid.New().String()
	purchaseTxID := xid.New().String()
	idempotencyKey := "refund-123"

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

	uc := refund.New(&mockTransactionRunner{}, txRepo, walletSvc, entitleSvc, &mockIDGenerator{}, &mockTimeProvider{})

	// First refund
	tx1, err := uc.Execute(context.Background(), userID, offeringID, &idempotencyKey)
	if err != nil {
		t.Fatalf("expected no error on first refund, got: %v", err)
	}

	// Second refund with same idempotency key should return cached result
	tx2, err := uc.Execute(context.Background(), userID, offeringID, &idempotencyKey)
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
