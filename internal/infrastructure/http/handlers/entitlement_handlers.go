package handlers

import (
	"net/http"

	offeringsDomain "github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/offerings/domain"
	"github.com/rs/xid"
)

type EntitlementHandlers struct {
	entitlementRepo offeringsDomain.EntitlementRepository
	offeringRepo    offeringsDomain.OfferingRepository
}

func NewEntitlementHandlers(
	entitlementRepo offeringsDomain.EntitlementRepository,
	offeringRepo offeringsDomain.OfferingRepository,
) *EntitlementHandlers {
	return &EntitlementHandlers{
		entitlementRepo: entitlementRepo,
		offeringRepo:    offeringRepo,
	}
}

// GetUserEntitlements handles GET /users/{user_id}/entitlements
func (h *EntitlementHandlers) GetUserEntitlements(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.PathValue("user_id")
	if _, err := xid.FromString(userIDStr); err != nil {
		JSONErrorWithCode(w, "invalid user_id", "INVALID_ID", http.StatusBadRequest)
		return
	}

	entitlements, err := h.entitlementRepo.GetByUserID(r.Context(), userIDStr)
	if err != nil {
		JSONError(w, err)
		return
	}

	// Enrich with offering details
	result := make([]map[string]interface{}, 0, len(entitlements))
	for _, ent := range entitlements {
		offering, err := h.offeringRepo.GetByID(r.Context(), ent.OfferingID)
		if err != nil {
			continue // Skip if offering not found
		}

		entData := map[string]interface{}{
			"id":             ent.ID,
			"offering_id":    ent.OfferingID,
			"offering_name":  offering.Name,
			"status":         ent.Status,
			"granted_at":     ent.GrantedAt,
			"transaction_id": ent.TransactionID,
		}

		if ent.RevokedAt != nil {
			entData["revoked_at"] = ent.RevokedAt
		}

		result = append(result, entData)
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"user_id":      userIDStr,
		"entitlements": result,
		"count":        len(result),
	})
}
