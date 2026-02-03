package app

import (
	offeringsInfra "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/offerings/infrastructure"
	offeringsSvc "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/offerings/services"
	paymentsInfra "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/infrastructure"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/usecases/deposit"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/usecases/history"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/usecases/purchase"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/usecases/refund"
	usersInfra "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/users/infrastructure"
	usersSvc "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/users/services"
	walletsInfra "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/wallets/infrastructure"
	walletsSvc "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/wallets/services"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/database"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/http/handlers"
)

type Container struct {
	WalletHandlers      *handlers.WalletHandlers
	PaymentHandlers     *handlers.PaymentHandlers
	EntitlementHandlers *handlers.EntitlementHandlers
}

func NewContainer(db *database.DB) *Container {
	// Initialize transaction runner for use cases
	txRunner := database.NewTransactionRunner(db)

	// Initialize repositories
	userRepo := usersInfra.NewPostgresUserRepository(db)
	walletRepo := walletsInfra.NewPostgresWalletRepository(db)
	offeringRepo := offeringsInfra.NewPostgresOfferingRepository(db)
	entitlementRepo := offeringsInfra.NewPostgresEntitlementRepository(db)
	txRepo := paymentsInfra.NewPostgresTransactionRepository(db)

	// Initialize domain services
	walletSvc := walletsSvc.NewWalletService(walletRepo)
	userSvc := usersSvc.NewUserService(userRepo)
	offeringSvc := offeringsSvc.NewOfferingService(offeringRepo)
	entitleSvc := offeringsSvc.NewEntitlementService(entitlementRepo)

	// Initialize payment provider (mock)
	paymentProvider := paymentsInfra.NewMockPaymentProvider()

	// Initialize use cases
	depositUC := deposit.New(txRepo, walletSvc, userSvc, paymentProvider)
	purchaseUC := purchase.New(txRunner, txRepo, walletSvc, userSvc, offeringSvc, entitleSvc)
	refundUC := refund.New(txRunner, txRepo, walletSvc, entitleSvc)
	historyUC := history.New(txRepo, userSvc)

	// Initialize handlers
	walletHandlers := handlers.NewWalletHandlers(walletRepo, userRepo)
	paymentHandlers := handlers.NewPaymentHandlers(depositUC, purchaseUC, refundUC, historyUC)
	entitlementHandlers := handlers.NewEntitlementHandlers(entitlementRepo, offeringRepo)

	return &Container{
		WalletHandlers:      walletHandlers,
		PaymentHandlers:     paymentHandlers,
		EntitlementHandlers: entitlementHandlers,
	}
}
