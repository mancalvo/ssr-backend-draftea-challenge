package handlers

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/mancalvo/ssr-backend-draftea-challenge/pkg/errors"
)

// Response helpers

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

type APIError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
	})
}

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

// DecodeJSON decodes request body into target struct
func DecodeJSON(r *http.Request, target interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		return apperrors.ErrInvalidInput
	}
	return nil
}
