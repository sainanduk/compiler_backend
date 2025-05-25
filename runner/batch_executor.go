package runner

import (
	"context"
	"fmt"
	"online-compiler/models"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ExecuteBatchInDocker executes code against multiple test cases in a single container
func ExecuteBatchInDocker(ctx context.Context, req models.BatchExecuteRequest) (map[string]string, error) {
	// Record start time
	startTime := time.Now()

	// Create unique directory for this execution
	execID := fmt.Sprintf("%d", time.Now().UnixNano())
	execDir := filepath.Join("sandbox", execID)

	// Get absolute path of execution directory
	absExecDir, err := filepath.Abs(execDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create execution directory
	if err := os.MkdirAll(execDir, 0777); err != nil {
		return nil, fmt.Errorf("failed to create execution directory: %w", err)
	}

	// Clean up execution directory when done
	defer os.RemoveAll(execDir)

	// Get language specification
	codeFile, _ := getLanguageSpec(req.Language)
	if codeFile == "" {
		return nil, fmt.Errorf("unsupported language: %s", req.Language)
	}

	// Write code to file
	filePath := filepath.Join(execDir, codeFile)
	if err := os.WriteFile(filePath, []byte(req.Code), 0644); err != nil {
		return nil, fmt.Errorf("failed to write code file: %w", err)
	}

	// Create test cases directory
	testCasesDir := filepath.Join(execDir, "testcases")
	if err := os.MkdirAll(testCasesDir, 0777); err != nil {
		return nil, fmt.Errorf("failed to create test cases directory: %w", err)
	}

	// Write test cases to files
	for _, tc := range req.TestCases {
		tcFilePath := filepath.Join(testCasesDir, tc.ID+".in")
		if err := os.WriteFile(tcFilePath, []byte(tc.Input), 0644); err != nil {
			return nil, fmt.Errorf("failed to write test case file: %w", err)
		}
	}

	// Create batch runner script based on language
	runnerScript := createBatchRunnerScript(req.Language, len(req.TestCases))
	runnerPath := filepath.Join(execDir, "run_tests.sh")
	if err := os.WriteFile(runnerPath, []byte(runnerScript), 0755); err != nil {
		return nil, fmt.Errorf("failed to write runner script: %w", err)
	}

	// Create container name
	containerName := fmt.Sprintf("compiler_batch_%s", execID)

	// Run the code inside the container with resource limits
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--name", containerName,
		"--memory=512m",         // Memory limit
		"--cpus=1",              // CPU limit
		"--network=none",        // No network access
		"--pids-limit=100",      // Process limit
		"--ulimit", "nproc=100", // Set process limit via ulimit
		"--stop-timeout=5", // Force stop after 5 seconds if not responding
		"-v", absExecDir+":/code",
		"compiler-image",
		"sh", "-c", "cd /code && ./run_tests.sh")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's a compilation error
		compileErrorPath := filepath.Join(execDir, "compile_error.txt")
		if _, statErr := os.Stat(compileErrorPath); statErr == nil {
			// Read compilation error
			compileError, readErr := os.ReadFile(compileErrorPath)
			if readErr == nil {
				// Return compilation error for all test cases
				results := make(map[string]string)
				for _, tc := range req.TestCases {
					results[tc.ID] = "Compilation error: " + string(compileError)
				}
				return results, nil
			}
		}
		return nil, fmt.Errorf("execution failed: %w\nOutput: %s", err, string(output))
	}

	// Parse results from output files
	results := make(map[string]string)
	for _, tc := range req.TestCases {
		outputPath := filepath.Join(testCasesDir, tc.ID+".out")
		outputBytes, err := os.ReadFile(outputPath)
		if err != nil {
			results[tc.ID] = fmt.Sprintf("Failed to read output: %v", err)
		} else {
			results[tc.ID] = string(outputBytes)
		}
	}

	// Get memory usage
	executeReq := models.ExecuteRequest{
		Language: req.Language,
		Code:     req.Code,
		Input:    req.TestCases[0].Input, // Use the first test case input for memory stats
	}
	memoryUsage, err := GetContainerStats(ctx, executeReq)
	if err != nil {
		return results, nil
	}

	// Append memory usage to results
	for id, result := range results {
		results[id] = fmt.Sprintf("%s\nMemory Used: %d KB", result, memoryUsage.MemoryUsed)
	}

	// Calculate execution time
	executionTime := time.Since(startTime).Milliseconds()

	// Append execution time to results
	for id, result := range results {
		results[id] = fmt.Sprintf("%s\nExecution Time: %d ms", result, executionTime)
	}

	return results, nil
}

// createBatchRunnerScript creates a shell script to run all test cases
func createBatchRunnerScript(language string, numTestCases int) string {
	var sb strings.Builder

	sb.WriteString("#!/bin/sh\n\n")

	// Compile code if needed
	switch language {
	case "java":
		sb.WriteString("javac /code/Main.java\n")
		sb.WriteString("if [ $? -ne 0 ]; then\n")
		sb.WriteString("  echo \"Compilation error\" > /code/compile_error.txt\n")
		sb.WriteString("  exit 1\n")
		sb.WriteString("fi\n")
	case "cpp":
		sb.WriteString("g++ /code/main.cpp -o /code/a.out\n")
		sb.WriteString("if [ $? -ne 0 ]; then\n")
		sb.WriteString("  echo \"Compilation error\" > /code/compile_error.txt\n")
		sb.WriteString("  exit 1\n")
		sb.WriteString("fi\n")
	case "c":
		sb.WriteString("gcc /code/main.c -o /code/a.out\n")
		sb.WriteString("if [ $? -ne 0 ]; then\n")
		sb.WriteString("  echo \"Compilation error\" > /code/compile_error.txt\n")
		sb.WriteString("  exit 1\n")
		sb.WriteString("fi\n")
	}

	// Create a function to run a single test case with timeout
	sb.WriteString(`
run_test_case() {
    id=$1
    echo "Running test case $id"
    timeout 5s sh -c "cat /code/testcases/$id.in | `)

	// Add language-specific execution command
	switch language {
	case "python":
		sb.WriteString("python3 /code/main.py")
	case "java":
		sb.WriteString("java -cp /code Main")
	case "cpp", "c":
		sb.WriteString("/code/a.out")
	case "javascript":
		sb.WriteString("node /code/main.js")
	case "go":
		sb.WriteString("go run /code/main.go")
	}

	sb.WriteString(`" > /code/testcases/$id.out 2>&1
    exit_code=$?
    if [ $exit_code -eq 124 ]; then
        echo "Execution timed out. Your code may contain an infinite loop." > /code/testcases/$id.out
    elif [ $exit_code -ne 0 ]; then
        echo "Execution failed with exit code $exit_code" >> /code/testcases/$id.out
    fi
}

`)

	// Run each test case in sequence
	for i := 0; i < numTestCases; i++ {
		sb.WriteString(fmt.Sprintf("run_test_case tc_%d\n", i))
	}

	return sb.String()
}
