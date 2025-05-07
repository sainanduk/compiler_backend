package models

import (
	"os"
	"strconv"
	"time"
)

// Config holds the application configuration
type Config struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	RateLimit    int
	RateWindow   time.Duration
	MaxWorkers   int
	MaxQueueSize int
}

// LoadConfig loads configuration from environment variables with defaults
func LoadConfig() *Config {
	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8001"
	}

	// Get timeouts from environment or use defaults
	readTimeout := getDurationEnv("READ_TIMEOUT", 30*time.Second)
	writeTimeout := getDurationEnv("WRITE_TIMEOUT", 30*time.Second)
	idleTimeout := getDurationEnv("IDLE_TIMEOUT", 120*time.Second)

	// Get rate limiting configuration
	rateLimit := getIntEnv("RATE_LIMIT", 100) // requests per window
	rateWindow := getDurationEnv("RATE_WINDOW", time.Minute)

	// Get worker pool configuration
	maxWorkers := getIntEnv("MAX_WORKERS", 10)
	maxQueueSize := getIntEnv("MAX_QUEUE_SIZE", 100)

	return &Config{
		Port:         port,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
		RateLimit:    rateLimit,
		RateWindow:   rateWindow,
		MaxWorkers:   maxWorkers,
		MaxQueueSize: maxQueueSize,
	}
}

// getDurationEnv gets a duration from environment variable with default
func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			return duration
		}
	}
	return defaultVal
}

// getIntEnv gets an integer from environment variable with default
func getIntEnv(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
