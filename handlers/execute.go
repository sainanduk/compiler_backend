package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"online-compiler/models"
	"online-compiler/runner"
	"time"
)

type ExecuteResponse struct {
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
	RequestID string `json:"request_id,omitempty"`
}

func ExecuteHandler(w http.ResponseWriter, r *http.Request) {
	// Set timeout context
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var req models.ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Language == "" || req.Code == "" {
		http.Error(w, "Language and code are required", http.StatusBadRequest)
		return
	}

	// Execute code with timeout
	output, err := runner.ExecuteInDocker(ctx, req)
	if err != nil {
		// Check if it's a timeout or rate limit error
		if err.Error() == "request cancelled: context deadline exceeded" {
			http.Error(w, "Request timed out", http.StatusGatewayTimeout)
			return
		}
		if err.Error() == "server is busy, please try again later" {
			http.Error(w, "Server is busy, please try again later", http.StatusTooManyRequests)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"output":     output,
		"status":     "success",
		"timestamp":  time.Now().Unix(),
		"request_id": time.Now().UnixNano(),
	})
}

func SubmitHandler(w http.ResponseWriter, r *http.Request) {
	// Set timeout context
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var req models.ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Language == "" || req.Code == "" {
		http.Error(w, "Language and code are required", http.StatusBadRequest)
		return
	}

	// Execute code with timeout
	output, err := runner.ExecuteInDocker(ctx, req)
	if err != nil {
		// Check if it's a timeout or rate limit error
		if err.Error() == "request cancelled: context deadline exceeded" {
			http.Error(w, "Request timed out", http.StatusGatewayTimeout)
			return
		}
		if err.Error() == "server is busy, please try again later" {
			http.Error(w, "Server is busy, please try again later", http.StatusTooManyRequests)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"output":     output,
		"status":     "success",
		"timestamp":  time.Now().Unix(),
		"request_id": time.Now().UnixNano(),
	})
}

func validateRequest(req models.ExecuteRequest) error {
	// Check language
	switch req.Language {
	case "python", "go", "cpp", "c":
		// Valid language
	default:
		return fmt.Errorf("unsupported language: %s", req.Language)
	}

	// Check code size
	if len(req.Code) == 0 {
		return fmt.Errorf("code cannot be empty")
	}
	if len(req.Code) > 1024*1024 { // 1MB limit
		return fmt.Errorf("code size exceeds maximum limit of 1MB")
	}

	// Additional validation for submissions
	if req.Input != "" && len(req.Input) > 1024*1024 { // 1MB limit for input
		return fmt.Errorf("input size exceeds maximum limit of 1MB")
	}

	return nil
}

func sendErrorResponse(w http.ResponseWriter, message string, status int, requestID string) {
	response := ExecuteResponse{
		Status:    "error",
		Error:     message,
		Timestamp: time.Now().Unix(),
		RequestID: requestID,
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}
