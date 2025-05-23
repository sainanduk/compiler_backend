package runner

import (
	"context"
	"fmt"
	"log"
	"online-compiler/models"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// ContainerStats represents the resource usage of a container
type ContainerStats struct {
	MemoryUsed int64 `json:"memory_used_kb"`
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
    case "java":
        return "Main.java", "javac /code/Main.java && echo \"$INPUT\" | java -cp /code Main"
    case "cpp":
        return "main.cpp", "g++ /code/main.cpp -o /code/a.out && echo \"$INPUT\" | /code/a.out"
    case "c":
        return "main.c", "gcc /code/main.c -o /code/a.out && echo \"$INPUT\" | /code/a.out"
    case "javascript":
        return "main.js", "echo \"$INPUT\" | node /code/main.js"
    case "go":
        return "main.go", "echo \"$INPUT\" | go run /code/main.go"
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

	// Check if Docker is available
	if err := checkDockerAvailability(); err != nil {
		stats.Success = false
		stats.ErrorMessage = fmt.Sprintf("Docker not available: %v", err)
		stats.EndTime = time.Now()
		statsChan <- stats
		return "", fmt.Errorf("Docker not available: %w", err)
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

	// Create container name
	containerName := fmt.Sprintf("compiler_%s", execID)

	// Create a channel to signal when the command is done
	done := make(chan error, 1)
	var output []byte
	var cmdErr error

	// Run the code inside the container with resource limits
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--name", containerName,
		"--memory=512m",         // Memory limit
		"--cpus=1",              // CPU limit
		"--network=none",        // No network access
		"--pids-limit=100",      // Process limit
		"--ulimit", "nproc=100", // Set process limit via ulimit
		// Add timeout to handle infinite loops
		"--stop-timeout=20",     // Force stop after 20 seconds if not responding
		"-e", fmt.Sprintf("INPUT=%s", req.Input), // Pass input as environment variable
		"-v", absExecDir+":/code",
		"compiler-image",
		"sh", "-c", runCmd)

	log.Printf("[DEBUG] Running Docker command: %s", strings.Join(cmd.Args, " "))

	// Run the command in a goroutine
	go func() {
		output, cmdErr = cmd.CombinedOutput()
		done <- cmdErr
	}()

	// Wait for either the command to finish or the context to timeout
	select {
	case err := <-done:
		// Command completed normally
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
	case <-ctx.Done():
		// Context timed out - force kill the container
		killCmd := exec.Command("docker", "kill", containerName)
		if err := killCmd.Run(); err != nil {
			log.Printf("[ERROR] Failed to kill container %s: %v", containerName, err)
		}
		stats.EndTime = time.Now()
		stats.Success = false
		stats.ErrorMessage = "execution timed out (possible infinite loop detected)"
		statsChan <- stats
		return "Execution timed out. Your code may contain an infinite loop or is taking too long to execute.", ctx.Err()
	}
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

// GetContainerStats retrieves the resource usage statistics for a container
func GetContainerStats(ctx context.Context, req models.ExecuteRequest) (ContainerStats, error) {
	// Get the container ID from the execution
	containerID := fmt.Sprintf("compiler_%d", time.Now().UnixNano())

	// Get container stats using docker stats
	cmd := exec.CommandContext(ctx, "docker", "stats", containerID, "--no-stream", "--format", "{{.MemUsage}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ContainerStats{}, fmt.Errorf("failed to get container stats: %w", err)
	}

	// Parse memory usage (format: "123.45MB / 512MB")
	memParts := strings.Split(strings.TrimSpace(string(output)), " / ")
	if len(memParts) != 2 {
		return ContainerStats{}, fmt.Errorf("invalid memory format")
	}
	memUsed := strings.TrimSpace(memParts[0])
	memUsed = strings.TrimSuffix(memUsed, "MB")
	memUsedKB, err := strconv.ParseFloat(memUsed, 64)
	if err != nil {
		return ContainerStats{}, fmt.Errorf("failed to parse memory usage: %w", err)
	}

	return ContainerStats{
		MemoryUsed: int64(memUsedKB * 1024), // Convert MB to KB
	}, nil
}

// checkDockerAvailability verifies that Docker is running and accessible
func checkDockerAvailability() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker is not running or not accessible: %w", err)
	}
	return nil
}
