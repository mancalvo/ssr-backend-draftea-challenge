package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/users/domain"
)

type UserService interface {
	IsActive(ctx context.Context, userID uuid.UUID) (bool, error)
}

type userService struct {
	userRepo domain.UserRepository
}

func NewUserService(userRepo domain.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func (s *userService) IsActive(ctx context.Context, userID uuid.UUID) (bool, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user.IsActive, nil
}
