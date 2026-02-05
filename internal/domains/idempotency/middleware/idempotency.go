package middleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/idempotency/domain"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/idempotency/services"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
	"github.com/mancalvo/ssr-backend-draftea-challenge/pkg/httputil"
)

const (
	// HeaderIdempotencyKey is the HTTP header for client-provided idempotency keys.
	HeaderIdempotencyKey = "Idempotency-Key"

	// HeaderIdempotentReplayed indicates the response is a replay of a previous request.
	HeaderIdempotentReplayed = "Idempotent-Replayed"

	// DefaultTTL is the default time-to-live for idempotency records.
	DefaultTTL = 1 * time.Hour
)

// Config holds configuration options for the idempotency middleware.
type Config struct {
	TTL time.Duration
}

// Option is a function that configures the middleware.
type Option func(*Config)

// WithTTL sets the time-to-live for idempotency records.
func WithTTL(ttl time.Duration) Option {
	return func(c *Config) {
		c.TTL = ttl
	}
}

// New creates an idempotency middleware that wraps handlers.
func New(svc services.Service, opts ...Option) func(http.Handler) http.Handler {
	cfg := Config{TTL: DefaultTTL}
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(next http.Handler) http.Handler {
		return &idempotencyHandler{
			svc:  svc,
			cfg:  cfg,
			next: next,
		}
	}
}

// idempotencyHandler implements http.Handler with idempotency logic.
type idempotencyHandler struct {
	svc  services.Service
	cfg  Config
	next http.Handler
}

func (h *idempotencyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := h.readBody(r)
	if err != nil {
		httputil.JSONErrorWithCode(w, "failed to read request body", "BODY_READ_ERROR", http.StatusBadRequest)
		return
	}

	key, requestHash := h.resolveKeys(r, body)

	record, isNew, err := h.svc.GetOrCreate(r.Context(), key, requestHash, h.cfg.TTL)
	if err != nil {
		h.handleGetOrCreateError(w, err)
		return
	}

	if !isNew {
		h.handleExistingRecord(w, record)
		return
	}

	h.processAndCaptureResponse(w, r, key)
}

// readBody reads and restores the request body.
func (h *idempotencyHandler) readBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// resolveKeys determines the idempotency key and request hash.
func (h *idempotencyHandler) resolveKeys(r *http.Request, body []byte) (key, requestHash string) {
	requestHash = h.svc.GenerateKey(r.Method, r.URL.Path, body)
	key = r.Header.Get(HeaderIdempotencyKey)
	if key == "" {
		key = requestHash
	}
	return key, requestHash
}

// handleGetOrCreateError handles errors from GetOrCreate.
func (h *idempotencyHandler) handleGetOrCreateError(w http.ResponseWriter, err error) {
	if errors.Is(err, apperrors.ErrIdempotencyKeyReused) {
		httputil.JSONErrorWithCode(w, "idempotency key already used with different request", "IDEMPOTENCY_KEY_REUSED", http.StatusUnprocessableEntity)
		return
	}
	httputil.JSONErrorWithCode(w, "idempotency check failed", "IDEMPOTENCY_ERROR", http.StatusInternalServerError)
}

// handleExistingRecord handles the case where a record already exists.
func (h *idempotencyHandler) handleExistingRecord(w http.ResponseWriter, record *domain.IdempotencyRecord) {
	if record.IsInProgress() {
		httputil.JSONErrorWithCode(w, "request with this idempotency key is already in progress", "IDEMPOTENCY_IN_PROGRESS", http.StatusConflict)
		return
	}

	// Return cached response
	w.Header().Set("Content-Type", record.ContentType)
	w.Header().Set(HeaderIdempotentReplayed, "true")
	w.WriteHeader(record.StatusCode)
	w.Write(record.ResponseBody)
}

// processAndCaptureResponse processes the request and captures the response for caching.
func (h *idempotencyHandler) processAndCaptureResponse(w http.ResponseWriter, r *http.Request, key string) {
	captured := httputil.NewResponseCapture(w)

	h.next.ServeHTTP(captured, r)

	contentType := captured.Header().Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}

	// Best effort - don't fail the request if we can't store the response
	_ = h.svc.Complete(r.Context(), key, captured.StatusCode, captured.Body.Bytes(), contentType)
}
