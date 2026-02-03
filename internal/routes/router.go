package routes

import (
	"net/http"

	offeringsInfra "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/offerings/infrastructure"
	offeringsSvc "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/offerings/services"
	paymentsInfra "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/infrastructure"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/usecases"
	usersInfra "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/users/infrastructure"
	usersSvc "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/users/services"
	walletsInfra "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/wallets/infrastructure"
	walletsSvc "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/wallets/services"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/database"
)

func NewRouter(db *database.DB) http.Handler {
	mux := http.NewServeMux()

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
	paymentUC := usecases.NewPaymentUseCases(
		txRunner,
		txRepo,
		walletSvc,
		userSvc,
		offeringSvc,
		entitleSvc,
		paymentProvider,
	)

	// Initialize handlers
	walletHandlers := NewWalletHandlers(walletRepo, userRepo)
	paymentHandlers := NewPaymentHandlers(paymentUC)
	entitlementHandlers := NewEntitlementHandlers(entitlementRepo, offeringRepo)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Wallet routes
	mux.HandleFunc("GET /wallets/{user_id}/balance", walletHandlers.GetBalance)

	// Payment routes
	mux.HandleFunc("POST /payments/deposit", paymentHandlers.Deposit)
	mux.HandleFunc("POST /payments/purchase", paymentHandlers.Purchase)
	mux.HandleFunc("POST /payments/refund", paymentHandlers.Refund)
	mux.HandleFunc("GET /payments/history/{user_id}", paymentHandlers.GetHistory)

	// Entitlement routes
	mux.HandleFunc("GET /users/{user_id}/entitlements", entitlementHandlers.GetUserEntitlements)

	return mux
}
