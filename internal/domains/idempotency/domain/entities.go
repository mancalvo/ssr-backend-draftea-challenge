package domain

import (
	"time"
)

// IdempotencyRecord represents a stored idempotency record for request deduplication.
type IdempotencyRecord struct {
	Key          string // Client-provided key or auto-generated hash
	RequestHash  string // Hash of method + path + body + userID
	StatusCode   int    // 0 means request is in progress
	ResponseBody []byte // Cached response body
	ContentType  string // Response content type
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

// IsInProgress returns true if the request is still being processed.
func (r *IdempotencyRecord) IsInProgress() bool {
	return r.StatusCode == 0
}

// IsExpired returns true if the record has expired based on the given current time.
func (r *IdempotencyRecord) IsExpired(now time.Time) bool {
	return now.After(r.ExpiresAt)
}
