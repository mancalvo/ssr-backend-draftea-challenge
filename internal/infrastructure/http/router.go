package http

import (
	"net/http"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/http/handlers"
)

func NewRouter(
	walletHandlers *handlers.WalletHandlers,
	paymentHandlers *handlers.PaymentHandlers,
	entitlementHandlers *handlers.EntitlementHandlers,
) http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		handlers.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
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

