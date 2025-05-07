package main

import (
	"context"
	"log"
	"net/http"
	"online-compiler/handlers"
	"online-compiler/middleware"
	"online-compiler/models"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Load configuration
	config := models.LoadConfig()

	// Create sandbox directory
	if err := os.MkdirAll("sandbox", 0777); err != nil {
		log.Fatalf("Failed to create sandbox directory: %v", err)
	}

	// Create a new server with timeouts
	server := &http.Server{
		Addr:           config.Port,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		IdleTimeout:    config.IdleTimeout,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// Create rate limiter
	rateLimiter := middleware.NewRateLimiter(config.RateLimit, config.RateWindow)

	// Set up routes with middleware
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Code execution endpoints with middleware chain
	executionHandler := http.HandlerFunc(handlers.RunCode)
	submissionHandler := http.HandlerFunc(handlers.SubmitCode)

	// Apply middleware chain
	mux.Handle("/execute", middleware.RecoverMiddleware(
		middleware.RequestIDMiddleware(
			middleware.CORSMiddleware(
				rateLimiter.RateLimit(executionHandler),
			),
		),
	))

	mux.Handle("/submit", middleware.RecoverMiddleware(
		middleware.RequestIDMiddleware(
			middleware.CORSMiddleware(
				rateLimiter.RateLimit(submissionHandler),
			),
		),
	))

	// Apply logging middleware to all routes
	handler := middleware.LoggingMiddleware(mux)
	server.Handler = handler

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on %s", config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	log.Println("Shutting down server...")
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited properly")
}
