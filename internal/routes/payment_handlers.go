package routes

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/usecases"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
)

type PaymentHandlers struct {
	paymentUC usecases.PaymentUseCases
}

func NewPaymentHandlers(paymentUC usecases.PaymentUseCases) *PaymentHandlers {
	return &PaymentHandlers{paymentUC: paymentUC}
}

// DepositRequest represents the deposit request body
type DepositRequest struct {
	UserID         string  `json:"user_id"`
	Amount         int64   `json:"amount"` // cents
	CardToken      string  `json:"card_token"`
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
}

// Deposit handles POST /payments/deposit
func (h *PaymentHandlers) Deposit(w http.ResponseWriter, r *http.Request) {
	var req DepositRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONError(w, err)
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		JSONErrorWithCode(w, "invalid user_id", "INVALID_ID", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		JSONErrorWithCode(w, "amount must be positive", "INVALID_AMOUNT", http.StatusBadRequest)
		return
	}

	if req.CardToken == "" {
		JSONErrorWithCode(w, "card_token is required", "MISSING_CARD_TOKEN", http.StatusBadRequest)
		return
	}

	tx, err := h.paymentUC.Deposit(r.Context(), userID, req.Amount, req.CardToken, req.IdempotencyKey)
	if err != nil {
		// Special case: wallet credit failed but payment was received
		// Return 202 with transaction data so user knows payment went through
		if errors.Is(err, apperrors.ErrWalletCreditFailed) && tx != nil {
			JSON(w, http.StatusAccepted, map[string]interface{}{
				"transaction_id":  tx.ID,
				"type":            tx.Type,
				"amount":          tx.Amount,
				"status":          tx.Status,
				"provider_ref":    tx.ProviderRef,
				"idempotency_key": tx.IdempotencyKey,
				"message":         "Payment received. Your balance will be updated shortly.",
				"created_at":      tx.CreatedAt,
			})
			return
		}
		JSONError(w, err)
		return
	}

	JSON(w, http.StatusCreated, map[string]interface{}{
		"transaction_id":  tx.ID,
		"type":            tx.Type,
		"amount":          tx.Amount,
		"status":          tx.Status,
		"idempotency_key": tx.IdempotencyKey,
		"created_at":      tx.CreatedAt,
	})
}

// PurchaseRequest represents the purchase request body
type PurchaseRequest struct {
	UserID         string  `json:"user_id"`
	OfferingID     string  `json:"offering_id"`
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
}

// Purchase handles POST /payments/purchase
func (h *PaymentHandlers) Purchase(w http.ResponseWriter, r *http.Request) {
	var req PurchaseRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONError(w, err)
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		JSONErrorWithCode(w, "invalid user_id", "INVALID_ID", http.StatusBadRequest)
		return
	}

	offeringID, err := uuid.Parse(req.OfferingID)
	if err != nil {
		JSONErrorWithCode(w, "invalid offering_id", "INVALID_ID", http.StatusBadRequest)
		return
	}

	tx, err := h.paymentUC.Purchase(r.Context(), userID, offeringID, req.IdempotencyKey)
	if err != nil {
		JSONError(w, err)
		return
	}

	JSON(w, http.StatusCreated, map[string]interface{}{
		"transaction_id":  tx.ID,
		"type":            tx.Type,
		"amount":          tx.Amount,
		"offering_id":     tx.OfferingID,
		"status":          tx.Status,
		"idempotency_key": tx.IdempotencyKey,
		"created_at":      tx.CreatedAt,
	})
}

// RefundRequest represents the refund request body
type RefundRequest struct {
	UserID         string  `json:"user_id"`
	OfferingID     string  `json:"offering_id"`
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
}

// Refund handles POST /payments/refund
func (h *PaymentHandlers) Refund(w http.ResponseWriter, r *http.Request) {
	var req RefundRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONErrorWithCode(w, "invalid request body", "PARSE_ERROR", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		JSONErrorWithCode(w, "invalid user_id", "INVALID_ID", http.StatusBadRequest)
		return
	}

	offeringID, err := uuid.Parse(req.OfferingID)
	if err != nil {
		JSONErrorWithCode(w, "invalid offering_id", "INVALID_ID", http.StatusBadRequest)
		return
	}

	refundTx, err := h.paymentUC.Refund(r.Context(), userID, offeringID, req.IdempotencyKey)
	if err != nil {
		// Handle partial success cases (202)
		if (errors.Is(err, apperrors.ErrWalletCreditFailed) || errors.Is(err, apperrors.ErrRevokeFailed)) && refundTx != nil {
			JSON(w, http.StatusAccepted, map[string]interface{}{
				"transaction_id":  refundTx.ID,
				"type":            refundTx.Type,
				"amount":          refundTx.Amount,
				"offering_id":     refundTx.OfferingID,
				"status":          refundTx.Status,
				"idempotency_key": refundTx.IdempotencyKey,
				"message":         err.Error(),
				"created_at":      refundTx.CreatedAt,
			})
			return
		}
		JSONError(w, err)
		return
	}

	JSON(w, http.StatusCreated, map[string]interface{}{
		"transaction_id":  refundTx.ID,
		"type":            refundTx.Type,
		"amount":          refundTx.Amount,
		"offering_id":     refundTx.OfferingID,
		"status":          refundTx.Status,
		"idempotency_key": refundTx.IdempotencyKey,
		"created_at":      refundTx.CreatedAt,
	})
}

// GetHistory handles GET /payments/history/{user_id}
func (h *PaymentHandlers) GetHistory(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.PathValue("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		JSONErrorWithCode(w, "invalid user_id", "INVALID_ID", http.StatusBadRequest)
		return
	}

	page, pageSize := parsePageParams(r)

	paginated, err := h.paymentUC.GetHistory(r.Context(), userID, page, pageSize)
	if err != nil {
		if err == apperrors.ErrNotFound {
			JSONErrorWithCode(w, "user not found", "USER_NOT_FOUND", http.StatusNotFound)
			return
		}
		JSONError(w, err)
		return
	}

	// Transform transactions for response
	txList := make([]map[string]interface{}, len(paginated.Transactions))
	for i, tx := range paginated.Transactions {
		txList[i] = map[string]interface{}{
			"id":          tx.ID,
			"type":        tx.Type,
			"amount":      tx.Amount,
			"status":      tx.Status,
			"offering_id": tx.OfferingID,
			"created_at":  tx.CreatedAt,
		}
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"transactions": txList,
		"pagination": map[string]interface{}{
			"page":        paginated.Page,
			"page_size":   paginated.PageSize,
			"total_count": paginated.TotalCount,
			"total_pages": paginated.TotalPages,
		},
	})
}
