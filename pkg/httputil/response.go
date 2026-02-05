// Package httputil provides shared HTTP response utilities.
package httputil

import (
	"bytes"
	"encoding/json"
	"net/http"

	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
)

// APIResponse is the standard API response structure.
type APIResponse struct {
	Success bool      `json:"success"`
	Data    any       `json:"data,omitempty"`
	Error   *APIError `json:"error,omitempty"`
}

// APIError represents an error in an API response.
type APIError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// JSON writes a successful JSON response with the given status code and data.
func JSON(w http.ResponseWriter, status int, data any) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
	})
}

// JSONError writes a JSON error response, deriving the HTTP status from the error.
func JSONError(w http.ResponseWriter, err error) {
	status := apperrors.HTTPStatusCode(err)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error: &APIError{
			Message: err.Error(),
		},
	})
}

// JSONErrorWithCode writes a JSON error response with an explicit status code and error code.
func JSONErrorWithCode(w http.ResponseWriter, message, code string, status int) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error: &APIError{
			Message: message,
			Code:    code,
		},
	})
}

// DecodeJSON decodes the request body JSON into the target struct.
func DecodeJSON(r *http.Request, target any) error {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		return apperrors.ErrInvalidInput
	}
	return nil
}

// ResponseCapture wraps http.ResponseWriter to capture the response status code and body.
// This is useful for middleware that needs to inspect or cache responses.
type ResponseCapture struct {
	http.ResponseWriter
	StatusCode int
	Body       *bytes.Buffer
}

// NewResponseCapture creates a new ResponseCapture wrapping the given ResponseWriter.
func NewResponseCapture(w http.ResponseWriter) *ResponseCapture {
	return &ResponseCapture{
		ResponseWriter: w,
		StatusCode:     http.StatusOK,
		Body:           &bytes.Buffer{},
	}
}

// WriteHeader captures the status code and writes it to the underlying ResponseWriter.
func (rc *ResponseCapture) WriteHeader(code int) {
	rc.StatusCode = code
	rc.ResponseWriter.WriteHeader(code)
}

// Write captures the body and writes it to the underlying ResponseWriter.
func (rc *ResponseCapture) Write(b []byte) (int, error) {
	rc.Body.Write(b)
	return rc.ResponseWriter.Write(b)
}
