package errors

import (
	"errors"
	"net/http"
)

// Domain errors
var (
	ErrNotFound           = errors.New("resource not found")
	ErrInsufficientFunds  = errors.New("insufficient funds")
	ErrInvalidInput       = errors.New("invalid input")
	ErrUserNotActive      = errors.New("user is not active")
	ErrAlreadyExists      = errors.New("resource already exists")
	ErrAlreadyOwned       = errors.New("offering already owned")
	ErrPaymentFailed      = errors.New("payment processing failed")
	ErrProviderError      = errors.New("external provider error")
	ErrWalletCreditFailed = errors.New("payment received, balance update pending")
	ErrRevokeFailed       = errors.New("refund credited, entitlement revoke pending")
)

// AppError wraps domain errors with HTTP status codes
type AppError struct {
	Err     error
	Message string
	Code    int
}

func (e *AppError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Err.Error()
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new application error
func NewAppError(err error, message string, code int) *AppError {
	return &AppError{
		Err:     err,
		Message: message,
		Code:    code,
	}
}

// ProviderDeclined represents a payment declined by the provider
type ProviderDeclined struct {
	Message string
}

func (e *ProviderDeclined) Error() string {
	return e.Message
}

func NewProviderDeclined(message string) error {
	return &ProviderDeclined{Message: message}
}

// HTTPStatusCode maps domain errors to HTTP status codes
func HTTPStatusCode(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrInsufficientFunds):
		return http.StatusBadRequest
	case errors.Is(err, ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, ErrUserNotActive):
		return http.StatusForbidden
	case errors.Is(err, ErrAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, ErrAlreadyOwned):
		return http.StatusConflict
	case errors.Is(err, ErrPaymentFailed):
		return http.StatusPaymentRequired
	case errors.Is(err, ErrProviderError):
		return http.StatusBadGateway
	case errors.Is(err, ErrWalletCreditFailed):
		return http.StatusAccepted // 202 - Payment received, balance updating
	case errors.Is(err, ErrRevokeFailed):
		return http.StatusAccepted // 202 - Refund credited, revoke pending
	default:
		return http.StatusInternalServerError
	}
}
