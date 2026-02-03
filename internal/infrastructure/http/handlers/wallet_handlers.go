package handlers

import (
	"net/http"
	"strconv"

	usersDomain "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/users/domain"
	walletsDomain "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/wallets/domain"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
	"github.com/rs/xid"
)

type WalletHandlers struct {
	walletRepo walletsDomain.WalletRepository
	userRepo   usersDomain.UserRepository
}

func NewWalletHandlers(walletRepo walletsDomain.WalletRepository, userRepo usersDomain.UserRepository) *WalletHandlers {
	return &WalletHandlers{
		walletRepo: walletRepo,
		userRepo:   userRepo,
	}
}

// GetBalance handles GET /wallets/{user_id}/balance
func (h *WalletHandlers) GetBalance(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.PathValue("user_id")
	if _, err := xid.FromString(userIDStr); err != nil {
		JSONErrorWithCode(w, "invalid user_id", "INVALID_ID", http.StatusBadRequest)
		return
	}

	// Validate user exists and is active
	user, err := h.userRepo.GetByID(r.Context(), userIDStr)
	if err != nil {
		JSONError(w, err)
		return
	}
	if !user.IsActive {
		JSONError(w, apperrors.ErrUserNotActive)
		return
	}

	wallet, err := h.walletRepo.GetByUserID(r.Context(), userIDStr)
	if err != nil {
		JSONError(w, err)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"user_id":      wallet.UserID,
		"balance":      wallet.Balance,
		"balance_mxn":  float64(wallet.Balance) / 100,
		"currency":     "MXN",
		"last_updated": wallet.UpdatedAt,
	})
}

// ParsePageParams extracts pagination parameters from query string
func ParsePageParams(r *http.Request) (page, pageSize int) {
	page = 1
	pageSize = 20

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 100 {
			pageSize = v
		}
	}

	return
}
