package main

import (
	"log"
	"net/http"
	"online-compiler/handlers"
	"online-compiler/middleware"
	"online-compiler/models"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	// Load configuration
	config := models.LoadConfig()

	// Create router
	r := mux.NewRouter()

	// Add middleware
	r.Use(middleware.LoggingMiddleware)
	r.Use(middleware.RecoveryMiddleware)
	r.Use(middleware.CORSMiddleware)
	r.Use(middleware.RequestIDMiddleware)
	r.Use(middleware.RateLimitMiddleware)

	// Add routes
	r.HandleFunc("/execute", handlers.ExecuteHandler).Methods("POST")
	r.HandleFunc("/submit", handlers.SubmitHandler).Methods("POST")

	// Create server with timeouts
	srv := &http.Server{
		Handler:      r,
		Addr:         config.Port,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server
	log.Printf("Server starting on %s", config.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
