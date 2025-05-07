package runner

import (
	"fmt"
	"os"
	"path/filepath"
)

func GetLanguageSpec(lang, container, code string) (filename, cmd string) {
	switch lang {
	case "python":
		return "main.py", "python3 /code/main.py"
	case "go":
		return "main.go", "go run /code/main.go"
	case "c":
		return "main.c", "gcc /code/main.c -o /code/a.out && /code/a.out"
	case "cpp":
		return "main.cpp", "g++ /code/main.cpp -o /code/a.out && /code/a.out"
	default:
		return "main.txt", "echo 'Unsupported Language'"
	}
}

func WriteCodeToFile(filename, content string) error {
	// Create sandbox directory if it doesn't exist
	sandboxDir := "./sandbox"
	if err := os.MkdirAll(sandboxDir, 0777); err != nil {
		return fmt.Errorf("failed to create sandbox directory: %w", err)
	}

	// Get absolute path
	absPath, err := filepath.Abs(sandboxDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Write the file
	path := filepath.Join(absPath, filename)
	return os.WriteFile(path, []byte(content), 0644)
}
