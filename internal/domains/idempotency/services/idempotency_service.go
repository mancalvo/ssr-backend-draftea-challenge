package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/domains/idempotency/domain"
	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
	timeprovider "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/providers/time"
)

// Service defines the interface for idempotency operations.
type Service interface {
	// GetOrCreate looks up an existing record or creates a new "in-progress" one.
	// Returns (record, isNew, error).
	GetOrCreate(ctx context.Context, key, requestHash string, ttl time.Duration) (*domain.IdempotencyRecord, bool, error)

	// Complete marks the request as complete with the response.
	Complete(ctx context.Context, key string, statusCode int, body []byte, contentType string) error

	// GenerateKey creates a deterministic key from request attributes.
	GenerateKey(method, path string, body []byte) string

	// Cleanup removes expired records in batches. Returns the number of deleted records.
	Cleanup(ctx context.Context, batchSize int) (int64, error)
}

type service struct {
	repo     domain.Repository
	timeProv timeprovider.Provider
}

// New creates a new idempotency service.
func New(repo domain.Repository, timeProv timeprovider.Provider) Service {
	return &service{
		repo:     repo,
		timeProv: timeProv,
	}
}

func (s *service) GetOrCreate(ctx context.Context, key, requestHash string, ttl time.Duration) (*domain.IdempotencyRecord, bool, error) {
	// Try to find existing valid record
	existing, err := s.findExistingRecord(ctx, key, requestHash)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}

	// No existing record, create new one
	return s.createNewRecord(ctx, key, requestHash, ttl)
}

// findExistingRecord looks up a record and validates it matches the request hash.
// Returns (record, nil) if found and valid, (nil, nil) if not found, or (nil, error) on mismatch/error.
func (s *service) findExistingRecord(ctx context.Context, key, requestHash string) (*domain.IdempotencyRecord, error) {
	existing, err := s.repo.Get(ctx, key)
	if errors.Is(err, apperrors.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Validate request hash matches
	if existing.RequestHash != requestHash {
		return nil, apperrors.ErrIdempotencyKeyReused
	}

	return existing, nil
}

// createNewRecord creates a new idempotency record, handling race conditions.
func (s *service) createNewRecord(ctx context.Context, key, requestHash string, ttl time.Duration) (*domain.IdempotencyRecord, bool, error) {
	record := s.buildRecord(key, requestHash, ttl)

	err := s.repo.Create(ctx, record)
	if err == nil {
		return record, true, nil
	}

	// Handle race condition: another request created the record
	if errors.Is(err, apperrors.ErrAlreadyExists) {
		return s.handleCreateConflict(ctx, key, requestHash)
	}

	return nil, false, err
}

// buildRecord constructs a new IdempotencyRecord.
func (s *service) buildRecord(key, requestHash string, ttl time.Duration) *domain.IdempotencyRecord {
	now := s.timeProv.Now()
	return &domain.IdempotencyRecord{
		Key:         key,
		RequestHash: requestHash,
		StatusCode:  0, // 0 indicates in-progress
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
		ContentType: "application/json",
	}
}

// handleCreateConflict handles the case where another request created the record during a race.
func (s *service) handleCreateConflict(ctx context.Context, key, requestHash string) (*domain.IdempotencyRecord, bool, error) {
	existing, err := s.repo.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}

	if existing.RequestHash != requestHash {
		return nil, false, apperrors.ErrIdempotencyKeyReused
	}

	return existing, false, nil
}

func (s *service) Complete(ctx context.Context, key string, statusCode int, body []byte, contentType string) error {
	record, err := s.repo.Get(ctx, key)
	if err != nil {
		return err
	}

	record.StatusCode = statusCode
	record.ResponseBody = body
	record.ContentType = contentType

	return s.repo.Update(ctx, record)
}

func (s *service) GenerateKey(method, path string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte(":"))
	h.Write([]byte(path))
	h.Write([]byte(":"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func (s *service) Cleanup(ctx context.Context, batchSize int) (int64, error) {
	return s.repo.DeleteExpiredBatch(ctx, batchSize)
}
