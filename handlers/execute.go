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

func RunCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	handleCodeExecution(w, r, false)
}

func SubmitCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	handleCodeExecution(w, r, true)
}

func handleCodeExecution(w http.ResponseWriter, r *http.Request, isSubmission bool) {
	w.Header().Set("Content-Type", "application/json")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Parse request
	var req models.ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, "Invalid request format", http.StatusBadRequest, "")
		return
	}

	// Validate request
	if err := validateRequest(req); err != nil {
		sendErrorResponse(w, err.Error(), http.StatusBadRequest, "")
		return
	}

	// Generate request ID
	requestID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Execute code
	output, err := runner.ExecuteInDocker(ctx, req)

	response := ExecuteResponse{
		Timestamp: time.Now().Unix(),
		RequestID: requestID,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			response.Status = "timeout"
			response.Error = "Execution timed out"
			w.WriteHeader(http.StatusGatewayTimeout)
		} else {
			response.Status = "error"
			response.Error = err.Error()
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		response.Status = "success"
		response.Output = output
	}

	json.NewEncoder(w).Encode(response)
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
