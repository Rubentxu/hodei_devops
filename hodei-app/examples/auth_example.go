package main

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/application/iam"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/infrastructure/config"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Initialize configuration
	authConfig := config.DefaultAuthConfig()
	
	// For demonstration purposes, we'll use in-memory Redis replacement
	// In a real application, you would use a real Redis instance
	authConfig.RedisOptions.Addr = "localhost:6379"
	
	// Initialize auth services
	authService, authMiddleware, authHandlers, err := config.InitializeAuthServices(authConfig)
	if err != nil {
		log.Fatalf("Failed to initialize auth services: %v", err)
	}
	
	// Create a new HTTP server mux
	mux := http.NewServeMux()
	
	// Set up auth routes
	authHandlers.SetupRoutes(mux, authMiddleware)
	
	// Set up a protected route that requires authentication
	mux.Handle("/api/protected", authMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get user information from context
		userID, _ := iam.GetUserID(r.Context())
		username, _ := iam.GetUsername(r.Context())
		
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"message": "This is a protected endpoint", "user_id": "%s", "username": "%s"}`, userID, username)
	})))
	
	// Set up a route that requires a specific role
	mux.Handle("/api/admin", authMiddleware.Middleware(
		authMiddleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"message": "This is an admin-only endpoint"}`)
		})),
	))
	
	// Create a sample user for testing (in a real application, this would be done through registration)
	createSampleUser(authService)
	
	// Create HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	
	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()
	
	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	log.Println("Shutting down server...")
	
	// Create a deadline for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	
	log.Println("Server exiting")
}

// createSampleUser creates a sample user for testing
func createSampleUser(authService iam.AuthService) {
	ctx := context.Background()
	
	// Create a regular user
	_, err := authService.Register(ctx, "user", "password", "acme-inc", "frontend", []string{"user"})
	if err != nil && err != model.ErrUserAlreadyExists {
		log.Printf("Failed to create sample user: %v", err)
	}
	
	// Create an admin user
	_, err = authService.Register(ctx, "admin", "admin_password", "acme-inc", "frontend", []string{"admin", "user"})
	if err != nil && err != model.ErrUserAlreadyExists {
		log.Printf("Failed to create sample admin user: %v", err)
	}
	
	log.Println("Sample users created")
}
