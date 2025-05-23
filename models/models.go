package models

// ExecuteRequest represents a code execution request
type ExecuteRequest struct {
	Code     string `json:"code"`
	Language string `json:"language"`
	Input    string `json:"input,omitempty"`
}

// TestInput represents a single test case input for batch execution
type TestInput struct {
	ID    string `json:"id"`
	Input string `json:"input"`
}

// BatchExecuteRequest represents a request to execute code against multiple test cases
type BatchExecuteRequest struct {
	Code      string      `json:"code"`
	Language  string      `json:"language"`
	TestCases []TestInput `json:"test_cases"`
}