package runner

import (
	"context"
	"fmt"
	"log"
	"online-compiler/models"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ExecutionStats tracks execution statistics
type ExecutionStats struct {
	StartTime    time.Time
	EndTime      time.Time
	Language     string
	CodeSize     int
	Success      bool
	ErrorMessage string
	RequestID    string
}

// ExecutionRequest represents a code execution request
type ExecutionRequest struct {
	ID       string
	Request  models.ExecuteRequest
	Response chan ExecutionResult
	Timeout  time.Duration
}

// ExecutionResult represents the result of code execution
type ExecutionResult struct {
	Output string
	Error  error
}

var (
	statsChan   = make(chan ExecutionStats, 1000)  // Buffer for stats
	requestChan = make(chan ExecutionRequest, 100) // Buffer for requests
	workerCount = 10                               // Number of concurrent workers
	workerWg    sync.WaitGroup

	// Rate limiting
	rateLimiter    = make(chan struct{}, 20) // Allow 20 concurrent requests
	requestTimeout = 30 * time.Second        // Default timeout for requests
)

func init() {
	// Start stats collector
	go collectStats()

	// Start worker pool
	for i := 0; i < workerCount; i++ {
		workerWg.Add(1)
		go worker()
	}
}

func worker() {
	defer workerWg.Done()
	for req := range requestChan {
		// Create a context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), req.Timeout)

		// Try to acquire rate limiter
		select {
		case rateLimiter <- struct{}{}:
			// Got rate limit token
			output, err := executeCodeWithContext(ctx, req.Request)
			req.Response <- ExecutionResult{
				Output: output,
				Error:  err,
			}
			<-rateLimiter // Release rate limit token
		case <-ctx.Done():
			// Context timed out or was cancelled
			req.Response <- ExecutionResult{
				Error: fmt.Errorf("request timed out or rate limit exceeded"),
			}
		}
		cancel()
	}
}

func collectStats() {
	for stats := range statsChan {
		log.Printf("[STATS] Request completed - ID: %s, Language: %s, Duration: %v, Success: %v, Error: %s",
			stats.RequestID,
			stats.Language,
			stats.EndTime.Sub(stats.StartTime),
			stats.Success,
			stats.ErrorMessage)
	}
}

func getLanguageSpec(language string) (string, string) {
	switch language {
	case "python":
		return "main.py", "echo \"$INPUT\" | python3 /code/main.py"
	case "go":
		return "main.go", "go run /code/main.go"
	case "cpp":
		return "main.cpp", "g++ /code/main.cpp -o /code/a.out && /code/a.out"
	case "c":
		return "main.c", "gcc /code/main.c -o /code/a.out && /code/a.out"
	default:
		return "", ""
	}
}

func executeCodeWithContext(ctx context.Context, req models.ExecuteRequest) (string, error) {
	stats := ExecutionStats{
		StartTime: time.Now(),
		Language:  req.Language,
		CodeSize:  len(req.Code),
		RequestID: fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	// Create unique directory for this execution
	execID := stats.RequestID
	execDir := filepath.Join("sandbox", execID)

	// Get absolute path of execution directory
	absExecDir, err := filepath.Abs(execDir)
	if err != nil {
		stats.Success = false
		stats.ErrorMessage = fmt.Sprintf("failed to get absolute path: %v", err)
		stats.EndTime = time.Now()
		statsChan <- stats
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create execution directory
	if err := os.MkdirAll(execDir, 0777); err != nil {
		stats.Success = false
		stats.ErrorMessage = fmt.Sprintf("failed to create execution directory: %v", err)
		stats.EndTime = time.Now()
		statsChan <- stats
		return "", fmt.Errorf("failed to create execution directory: %w", err)
	}

	// Clean up execution directory when done
	defer os.RemoveAll(execDir)

	log.Printf("[INFO] Processing request - ID: %s, Language: %s", execID, req.Language)

	codeFile, runCmd := getLanguageSpec(req.Language)
	filePath := filepath.Join(execDir, codeFile)

	// Write code to file in the unique directory
	if err := os.WriteFile(filePath, []byte(req.Code), 0644); err != nil {
		stats.Success = false
		stats.ErrorMessage = fmt.Sprintf("failed to write code file: %v", err)
		stats.EndTime = time.Now()
		statsChan <- stats
		return "", fmt.Errorf("failed to write code file: %w", err)
	}

	// Run the code inside the container with resource limits
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--memory=512m",         // Memory limit
		"--cpus=1",              // CPU limit
		"--network=none",        // No network access
		"--pids-limit=100",      // Process limit
		"--ulimit", "nproc=100", // Set process limit via ulimit
		"-e", fmt.Sprintf("INPUT=%s", req.Input), // Pass input as environment variable
		"-v", absExecDir+":/code",
		"compiler-image",
		"sh", "-c", runCmd)

	log.Printf("[DEBUG] Running Docker command: %s", strings.Join(cmd.Args, " "))

	output, err := cmd.CombinedOutput()

	stats.EndTime = time.Now()
	if err != nil {
		stats.Success = false
		stats.ErrorMessage = fmt.Sprintf("execution failed: %v", err)
		statsChan <- stats
		return string(output), fmt.Errorf("execution failed: %w\nOutput: %s", err, string(output))
	}

	stats.Success = true
	statsChan <- stats
	return string(output), nil
}

func ExecuteInDocker(ctx context.Context, req models.ExecuteRequest) (string, error) {
	// Create response channel
	responseChan := make(chan ExecutionResult, 1)

	// Generate unique request ID
	requestID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create execution request with timeout
	execReq := ExecutionRequest{
		ID:       requestID,
		Request:  req,
		Response: responseChan,
		Timeout:  requestTimeout,
	}

	// Try to send request to worker pool with timeout
	select {
	case requestChan <- execReq:
		// Request accepted
	case <-ctx.Done():
		return "", fmt.Errorf("request cancelled: %w", ctx.Err())
	default:
		// Queue is full
		return "", fmt.Errorf("server is busy, please try again later")
	}

	// Wait for response with context timeout
	select {
	case result := <-responseChan:
		return result.Output, result.Error
	case <-ctx.Done():
		return "", fmt.Errorf("request cancelled: %w", ctx.Err())
	}
}
