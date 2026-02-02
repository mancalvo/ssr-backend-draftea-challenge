package domain

import "context"

// PaymentProvider interface for external payment processing (deposits)
type PaymentProvider interface {
	ProcessPayment(ctx context.Context, amount int64, cardToken string, idempotencyKey string) (*ProviderResponse, error)
}

type ProviderResponse struct {
	Success      bool
	Reference    string
	ErrorMessage string
}
