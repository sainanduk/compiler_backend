package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"online-compiler/models"
	"online-compiler/runner"
	"strings"
	"time"
)

type ExecutionMetrics struct {
	ExecutionTime float64 `json:"execution_time_ms"` // Time taken in milliseconds
	MemoryUsed    int64   `json:"memory_used_kb"`    // Memory used in KB
}

type ExecuteResponse struct {
	Output    string           `json:"output"`
	Error     string           `json:"error,omitempty"`
	Status    string           `json:"status"`
	Timestamp int64            `json:"timestamp"`
	RequestID string           `json:"request_id,omitempty"`
	Metrics   ExecutionMetrics `json:"metrics,omitempty"`
}

func ExecuteHandler(w http.ResponseWriter, r *http.Request) {
	// Set timeout context
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)  // Reduced from 30 to 20 seconds
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

	// Start timing
	startTime := time.Now()

	// Execute code with timeout
	output, err := runner.ExecuteInDocker(ctx, req)

	// Calculate execution time
	executionTime := time.Since(startTime).Seconds() * 1000 // Convert to milliseconds

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

	// Get container stats
	containerStats, err := runner.GetContainerStats(ctx, req)
	if err != nil {
		// Log the error but continue with the response
		fmt.Printf("Error getting container stats: %v\n", err)
	}

	// Prepare response
	response := ExecuteResponse{
		Output:    output,
		Status:    "success",
		Timestamp: time.Now().Unix(),
		RequestID: fmt.Sprintf("%d", time.Now().UnixNano()),
		Metrics: ExecutionMetrics{
			ExecutionTime: executionTime,
			MemoryUsed:    containerStats.MemoryUsed,
		},
	}

	// Log the response details
	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	fmt.Printf("\n===== EXECUTE RESPONSE =====\n%s\n============================\n", string(responseJSON))

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// TestCase represents a single test case for code submission
type TestCase struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
}

// SubmitRequest extends ExecuteRequest with test cases
type SubmitRequest struct {
	models.ExecuteRequest
	TestCases []TestCase `json:"test_cases"`
}

// TestCaseResult represents the result of a single test case
type TestCaseResult struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	ActualOutput   string `json:"actual_output"`
	Passed         bool   `json:"passed"`
}

// SubmitResponse represents the response for a code submission
type SubmitResponse struct {
	Status       string          `json:"status"`
	TotalCases   int             `json:"total_cases"`
	PassedCases  int             `json:"passed_cases"`
	Results      []TestCaseResult `json:"results"`
	ExecutionTime float64        `json:"execution_time_ms"`
	Timestamp    int64           `json:"timestamp"`
	RequestID    string          `json:"request_id,omitempty"`
}

func SubmitHandler(w http.ResponseWriter, r *http.Request) {
	// Set timeout context
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second) // Increased timeout for multiple test cases
	defer cancel()

	var req SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Log the request details
	requestJSON, _ := json.MarshalIndent(req, "", "  ")
	fmt.Printf("\n===== SUBMIT REQUEST =====\n%s\n==========================\n", string(requestJSON))

	// Validate request
	if req.Language == "" || req.Code == "" {
		http.Error(w, "Language and code are required", http.StatusBadRequest)
		return
	}

	if len(req.TestCases) == 0 {
		http.Error(w, "At least one test case is required", http.StatusBadRequest)
		return
	}

	// Limit the number of test cases to prevent abuse
	maxTestCases := 100
	if len(req.TestCases) > maxTestCases {
		http.Error(w, fmt.Sprintf("Too many test cases. Maximum allowed: %d", maxTestCases), http.StatusBadRequest)
		return
	}

	// Start timing
	startTime := time.Now()

	// Process test cases in batches
	results := make([]TestCaseResult, len(req.TestCases))
	passedCount := 0

	// Create a batch execution request
	batchReq := models.BatchExecuteRequest{
		Code:      req.Code,
		Language:  req.Language,
		TestCases: make([]models.TestInput, len(req.TestCases)),
	}

	// Prepare test cases for batch execution
	for i, tc := range req.TestCases {
		batchReq.TestCases[i] = models.TestInput{
			ID:    fmt.Sprintf("tc_%d", i),
			Input: tc.Input,
		}
	}

	// Execute all test cases in a single container
	batchResults, err := runner.ExecuteBatchInDocker(ctx, batchReq)
	
	if err != nil {
		// If the entire batch failed, mark all test cases as failed
		for i, tc := range req.TestCases {
			results[i] = TestCaseResult{
				Input:          tc.Input,
				ExpectedOutput: tc.ExpectedOutput,
				ActualOutput:   fmt.Sprintf("Execution error: %v", err),
				Passed:         false,
			}
		}
	} else {
		// Process results for each test case
		for i, tc := range req.TestCases {
			result := TestCaseResult{
				Input:          tc.Input,
				ExpectedOutput: tc.ExpectedOutput,
				ActualOutput:   batchResults[fmt.Sprintf("tc_%d", i)],
				Passed:         false,
			}

			// Check for timeout or error in this specific test case
			if strings.Contains(result.ActualOutput, "execution timed out") {
				result.ActualOutput = "Execution timed out. Your code may contain an infinite loop."
			} else {
				// Normalize outputs for comparison
				normalizedExpected := strings.TrimSpace(tc.ExpectedOutput)
				normalizedActual := strings.TrimSpace(result.ActualOutput)
				
				// Remove trailing newlines that might be added by different languages
				normalizedExpected = strings.TrimRight(normalizedExpected, "\n\r")
				normalizedActual = strings.TrimRight(normalizedActual, "\n\r")
				
				// Check if output matches expected output
				if normalizedActual == normalizedExpected {
					result.Passed = true
					passedCount++
				}
			}
			
			results[i] = result
		}
	}

	// Calculate execution time
	executionTime := time.Since(startTime).Seconds() * 1000 // Convert to milliseconds

	// Prepare response
	response := SubmitResponse{
		Status:        "success",
		TotalCases:    len(req.TestCases),
		PassedCases:   passedCount,
		Results:       results,
		ExecutionTime: executionTime,
		Timestamp:     time.Now().Unix(),
		RequestID:     fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	// Log the response details
	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	fmt.Printf("\n===== SUBMIT RESPONSE =====\n%s\n===========================\n", string(responseJSON))

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func validateRequest(req models.ExecuteRequest) error {
	// Check language
	switch req.Language {
	case "python", "java", "cpp", "c", "javascript", "go":
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

