package infrastructure

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/payments/domain"
)

// MockPaymentProvider simulates an external payment provider
type MockPaymentProvider struct {
	// FailureRate controls the probability of payment failure (0.0 to 1.0)
	FailureRate float64
}

func NewMockPaymentProvider() *MockPaymentProvider {
	return &MockPaymentProvider{
		FailureRate: 0.1, // 10% failure rate by default
	}
}

func (p *MockPaymentProvider) ProcessPayment(ctx context.Context, amount int64, cardToken string, idempotencyKey string) (*domain.ProviderResponse, error) {
	// Simulate processing delay
	time.Sleep(100 * time.Millisecond)

	// Simulate random failures
	if rand.Float64() < p.FailureRate {
		return &domain.ProviderResponse{
			Success:      false,
			ErrorMessage: "payment declined by provider",
		}, nil
	}

	// Simulate specific card token failures for testing
	switch cardToken {
	case "tok_decline":
		return &domain.ProviderResponse{
			Success:      false,
			ErrorMessage: "card declined",
		}, nil
	case "tok_insufficient":
		return &domain.ProviderResponse{
			Success:      false,
			ErrorMessage: "insufficient funds on card",
		}, nil
	case "tok_expired":
		return &domain.ProviderResponse{
			Success:      false,
			ErrorMessage: "card expired",
		}, nil
	}

	// Success case - include idempotency key in reference for reconciliation
	reference := fmt.Sprintf("mock_%d_%s", time.Now().UnixNano(), idempotencyKey[:min(16, len(idempotencyKey))])

	return &domain.ProviderResponse{
		Success:   true,
		Reference: reference,
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
