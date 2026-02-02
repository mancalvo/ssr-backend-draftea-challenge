package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/offerings/domain"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
)

type EntitlementService interface {
	GrantAccess(ctx context.Context, userID, offeringID, transactionID uuid.UUID) error
	RevokeAccess(ctx context.Context, userID, offeringID uuid.UUID) error
	HasActiveAccess(ctx context.Context, userID, offeringID uuid.UUID) (bool, error)
	GetActiveEntitlementForOffering(ctx context.Context, userID, offeringID uuid.UUID) (transactionID uuid.UUID, err error)
}

type entitlementService struct {
	entitlementRepo domain.EntitlementRepository
}

func NewEntitlementService(entitlementRepo domain.EntitlementRepository) EntitlementService {
	return &entitlementService{entitlementRepo: entitlementRepo}
}

func (s *entitlementService) GrantAccess(ctx context.Context, userID, offeringID, transactionID uuid.UUID) error {
	entitlement := &domain.Entitlement{
		ID:            uuid.New(),
		UserID:        userID,
		OfferingID:    offeringID,
		TransactionID: transactionID,
		Status:        domain.EntitlementActive,
		GrantedAt:     time.Now(),
	}
	return s.entitlementRepo.Create(ctx, entitlement)
}

func (s *entitlementService) RevokeAccess(ctx context.Context, userID, offeringID uuid.UUID) error {
	entitlements, err := s.entitlementRepo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}

	for _, e := range entitlements {
		if e.OfferingID == offeringID && e.Status == domain.EntitlementActive {
			return s.entitlementRepo.Revoke(ctx, e.ID)
		}
	}

	return apperrors.ErrNotFound
}

func (s *entitlementService) HasActiveAccess(ctx context.Context, userID, offeringID uuid.UUID) (bool, error) {
	entitlements, err := s.entitlementRepo.GetByUserID(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, e := range entitlements {
		if e.OfferingID == offeringID && e.Status == domain.EntitlementActive {
			return true, nil
		}
	}

	return false, nil
}

func (s *entitlementService) GetActiveEntitlementForOffering(ctx context.Context, userID, offeringID uuid.UUID) (uuid.UUID, error) {
	entitlements, err := s.entitlementRepo.GetByUserID(ctx, userID)
	if err != nil {
		return uuid.Nil, err
	}

	for _, e := range entitlements {
		if e.OfferingID == offeringID && e.Status == domain.EntitlementActive {
			return e.TransactionID, nil
		}
	}

	return uuid.Nil, apperrors.ErrNotFound
}
