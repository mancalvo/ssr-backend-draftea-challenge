package http

import (
	"net/http"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/http/handlers"
	"github.com/mancalvo/ssr-backend-draftea-challenge/pkg/httputil"
)

// RouterConfig holds dependencies for the router.
type RouterConfig struct {
	WalletHandlers      *handlers.WalletHandlers
	PaymentHandlers     *handlers.PaymentHandlers
	EntitlementHandlers *handlers.EntitlementHandlers
	IdempotencyMw       Middleware // Optional, nil = disabled
}

func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		httputil.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Read-only routes (no idempotency needed)
	mux.HandleFunc("GET /wallets/{user_id}/balance", cfg.WalletHandlers.GetBalance)
	mux.HandleFunc("GET /payments/history/{user_id}", cfg.PaymentHandlers.GetHistory)
	mux.HandleFunc("GET /users/{user_id}/entitlements", cfg.EntitlementHandlers.GetUserEntitlements)

	// Mutating payment routes - with idempotency middleware
	idempotentRoutes := []struct {
		pattern string
		handler http.HandlerFunc
	}{
		{"POST /payments/deposit", cfg.PaymentHandlers.Deposit},
		{"POST /payments/purchase", cfg.PaymentHandlers.Purchase},
		{"POST /payments/refund", cfg.PaymentHandlers.Refund},
	}

	for _, route := range idempotentRoutes {
		handler := http.Handler(route.handler)
		if cfg.IdempotencyMw != nil {
			handler = cfg.IdempotencyMw(handler)
		}
		mux.Handle(route.pattern, handler)
	}

	return mux
}
