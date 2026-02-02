package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/offerings/domain"
)

type OfferingService interface {
	GetPrice(ctx context.Context, offeringID uuid.UUID) (priceCents int64, err error)
	IsAvailable(ctx context.Context, offeringID uuid.UUID) (bool, error)
}

type offeringService struct {
	offeringRepo domain.OfferingRepository
}

func NewOfferingService(offeringRepo domain.OfferingRepository) OfferingService {
	return &offeringService{offeringRepo: offeringRepo}
}

func (s *offeringService) GetPrice(ctx context.Context, offeringID uuid.UUID) (int64, error) {
	offering, err := s.offeringRepo.GetByID(ctx, offeringID)
	if err != nil {
		return 0, err
	}
	return offering.PriceCents, nil
}

func (s *offeringService) IsAvailable(ctx context.Context, offeringID uuid.UUID) (bool, error) {
	offering, err := s.offeringRepo.GetByID(ctx, offeringID)
	if err != nil {
		return false, err
	}
	return offering.IsActive, nil
}
