package main

import (
	"context"
	"database/sql"
	"dev.rubentxu.hodei-devops/hodei-app/config"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/migrations"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/worker"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/worker/factories"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/application/iam"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"

	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// MongoDB related imports
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	// ID Generator
	generator_id "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository"

	// IAM repository adapters
	iamRepo "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/iam"

	// Resource pool adapters
	resourcepool "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/resource_pool"
	adapters "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/resource"
)

func main() {
	// Cargar configuración
	cfg := config.Load()
	log.Printf("Configuración cargada: %+v", cfg)

	// Initialize MongoDB connection
	mongoDB, err := initializeMongoDB()
	if err != nil {
		log.Fatalf("Error initializing MongoDB: %v", err)
	}

	// Initialize PostgreSQL connection
	postgresDB, err := initializePostgres()
	if err != nil {
		log.Fatalf("Error initializing PostgreSQL: %v", err)
	}
	defer postgresDB.Close()

	// Initialize ID Generator
	idGenerator := generator_id.NewIDGenerator("uuid")

	// Initialize IAM repositories and services
	iamServices, err := initializeIAMServices(mongoDB, idGenerator)
	if err != nil {
		log.Fatalf("Error initializing IAM services: %v", err)
	}

	// Initialize Resource Pool repositories and services
	resourcePoolServices, err := initializeResourcePoolServices(postgresDB, &cfg)
	if err != nil {
		log.Fatalf("Error initializing resource pool services: %v", err)
	}

	// Create a mock task execution service for now
	// In a real implementation, you would create a proper task execution service
	taskExecService := &ports.TaskExecutionService{}

	// Initialize Worker
	workerFactory := factories.NewWorkerInstanceFactory(cfg.GRPC)
	workerInstance := worker.NewWorker(
		cfg.WorkerName,
		cfg.MaxConcurrentTasks,
		workerFactory,
		taskExecService,
	)

	// Initialize HTTP handlers and routes
	mux := http.NewServeMux()

	// Set up auth routes
	setupAuthRoutes(mux, iamServices)

	// Set up resource pool routes
	setupResourcePoolRoutes(mux, resourcePoolServices.ResourcePoolManager)

	// Set up worker routes
	setupWorkerRoutes(mux, workerInstance)

	// Start HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
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

// IAM Services struct to hold all IAM-related services
type IAMServices struct {
	IdentityService      *iam.IdentityService
	AuthService          iam.AuthService
	AuthorizationService *iam.AuthorizationService
	TokenService         ports.TokenService
}

// Initialize IAM services
func initializeIAMServices(mongoDB *mongo.Database, idGenerator ports.IDGenerator) (*IAMServices, error) {
	// Create IAM factory
	iamFactory := iamRepo.NewFactory(mongoDB, idGenerator)

	// Create services
	identityService := iamFactory.CreateIdentityService()
	authorizationService := iamFactory.CreateAuthorizationService()

	// For now, we'll return a minimal implementation
	// In a real implementation, you would create proper token service and auth service
	// This is just a placeholder until those services are implemented
	return &IAMServices{
		IdentityService:      identityService,
		AuthorizationService: authorizationService,
		AuthService:          nil, // Will be implemented later
		TokenService:         nil, // Will be implemented later
	}, nil
}

// ResourcePoolServices struct to hold all resource pool related services
type ResourcePoolServices struct {
	ResourcePoolManager *resource.ResourcePoolManager
}

// Initialize Resource Pool services
func initializeResourcePoolServices(db *sql.DB, cfg *config.Config) (*ResourcePoolServices, error) {
	// Create repositories
	resourcePoolReadRepo := resourcepool.NewResourcePoolReadRepository(db)
	resourcePoolWriteRepo := resourcepool.NewResourcePoolWriteRepository(db)

	// Create factory
	poolFactory := adapters.NewDefaultResourcePoolFactory()

	// Create manager
	resourcePoolManager, err := resource.NewResourcePoolManager(resourcePoolReadRepo, resourcePoolWriteRepo, poolFactory)
	if err != nil {
		return nil, fmt.Errorf("error creating ResourcePoolManager: %v", err)
	}

	// Configure default pool if enabled
	if cfg.DefaultDockerResourcePool {
		if err := setupDefaultPool(resourcePoolManager, poolFactory); err != nil {
			return nil, fmt.Errorf("error configuring default pool: %v", err)
		}
	}

	return &ResourcePoolServices{
		ResourcePoolManager: resourcePoolManager,
	}, nil
}

// Setup auth routes
func setupAuthRoutes(mux *http.ServeMux, iamServices *IAMServices) {
	// Register route
	mux.HandleFunc("/api/auth/register", handleRegister)

	// Login route
	mux.HandleFunc("/api/auth/login", handleLogin)

	// Logout route
	mux.HandleFunc("/api/auth/logout", handleLogout)

	// Refresh token route
	mux.HandleFunc("/api/auth/refresh", handleRefreshToken)

	// User management routes
	mux.HandleFunc("/api/users", handleListUsers)
	mux.HandleFunc("/api/users/", handleGetUser)
}

// Setup resource pool routes
func setupResourcePoolRoutes(mux *http.ServeMux, resourcePoolManager *resource.ResourcePoolManager) {
	// List resource pools
	mux.HandleFunc("/api/resource-pools", handleListResourcePools)

	// Get resource pool by ID
	mux.HandleFunc("/api/resource-pools/", handleGetResourcePool)

	// Create resource pool
	mux.HandleFunc("/api/resource-pools", handleCreateResourcePool)

	// Update resource pool
	mux.HandleFunc("/api/resource-pools/", handleUpdateResourcePool)

	// Delete resource pool
	mux.HandleFunc("/api/resource-pools/", handleDeleteResourcePool)
}

// Setup worker routes
func setupWorkerRoutes(mux *http.ServeMux, worker ports.WorkerInstanceManager) {
	// Worker status
	mux.HandleFunc("/api/worker/status", handleWorkerStatus)

	// Worker tasks
	mux.HandleFunc("/api/worker/tasks", handleWorkerTasks)
}

// Helper function to set up default resource pool
func setupDefaultPool(resourcePoolManager *resource.ResourcePoolManager, poolFactory ports.ResourcePoolFactory) error {
	defaultDockerPoolConfig, err := poolFactory.CreateDefaultResourcePool()
	if err != nil {
		return err
	}
	return resourcePoolManager.SaveConfig(defaultDockerPoolConfig)
}

// Placeholder for handler implementations
func handleRegister(w http.ResponseWriter, r *http.Request) {
	// Implementation for user registration
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Registration endpoint")
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	// Implementation for user login
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Login endpoint")
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	// Implementation for user logout
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Logout endpoint")
}

func handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	// Implementation for token refresh
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Token refresh endpoint")
}

func handleListUsers(w http.ResponseWriter, r *http.Request) {
	// Implementation for listing users
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "List users endpoint")
}

func handleGetUser(w http.ResponseWriter, r *http.Request) {
	// Implementation for getting a specific user
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Get user endpoint")
}

func handleListResourcePools(w http.ResponseWriter, r *http.Request) {
	// Implementation for listing resource pools
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "List resource pools endpoint")
}

func handleGetResourcePool(w http.ResponseWriter, r *http.Request) {
	// Implementation for getting a specific resource pool
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Get resource pool endpoint")
}

func handleCreateResourcePool(w http.ResponseWriter, r *http.Request) {
	// Implementation for creating a resource pool
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "Create resource pool endpoint")
}

func handleUpdateResourcePool(w http.ResponseWriter, r *http.Request) {
	// Implementation for updating a resource pool
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Update resource pool endpoint")
}

func handleDeleteResourcePool(w http.ResponseWriter, r *http.Request) {
	// Implementation for deleting a resource pool
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Delete resource pool endpoint")
}

func handleWorkerStatus(w http.ResponseWriter, r *http.Request) {
	// Implementation for getting worker status
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Worker status endpoint")
}

func handleWorkerTasks(w http.ResponseWriter, r *http.Request) {
	// Implementation for managing worker tasks
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Worker tasks endpoint")
}
