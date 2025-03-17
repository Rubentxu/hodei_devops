package config

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/resource"
	usecases "dev.rubentxu.hodei-devops/hodei-app/internal/domain/application"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/application/iam"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"time"
)

// ServicesContainer holds all domain services for dependency injection.
// It provides access to business logic services throughout the application.
type ServicesContainer struct {
	// Resource and task management services
	ResourcePoolService     ports.ResourcePoolService
	TaskService             ports.TaskService
	TaskExecutionService    ports.TaskExecutionService
	WorkerDefinitionService ports.WorkerDefinitionService

	// Identity and Access Management services
	AuthService          ports.AuthService
	AuthorizationService *iam.AuthorizationService
	IdentityService      *iam.IdentityService
	TokenService         ports.TokenService
}

// InitializeServicesContainer creates and initializes all application services
// using the provided repositories container for data access.
func InitializeServicesContainer(repositoriesContainer *RepositoriesContainer) (*ServicesContainer, error) {
	// Initialize factories
	resourcePoolFactory := createResourcePoolFactory()
	idGenerator := createIDGenerator()

	// Initialize domain services
	resourcePoolService := createResourcePoolService(repositoriesContainer, resourcePoolFactory)
	taskService := createTaskService(repositoriesContainer)
	taskExecutionService := createTaskExecutionService(repositoriesContainer, idGenerator)
	workerDefinitionService := createWorkerDefinitionService(repositoriesContainer, idGenerator)

	// Initialize IAM services
	passwordHasher := createPasswordHasher()
	tokenService := createTokenService()

	authService := createAuthService(repositoriesContainer, tokenService, passwordHasher)
	authorizationService := createAuthorizationService(repositoriesContainer)
	identityService := createIdentityService(repositoriesContainer)

	return &ServicesContainer{
		ResourcePoolService:     resourcePoolService,
		TaskService:             taskService,
		TaskExecutionService:    taskExecutionService,
		WorkerDefinitionService: workerDefinitionService,
		AuthService:             authService,
		AuthorizationService:    authorizationService,
		IdentityService:         identityService,
		TokenService:            tokenService,
	}, nil
}

// Factory creation functions

// createResourcePoolFactory creates a factory for resource pool instances
func createResourcePoolFactory() ports.ResourcePoolFactory {
	return resource.NewDefaultResourcePoolFactory()
}

// Domain service creation functions

// createResourcePoolService creates the resource pool management service
func createResourcePoolService(rc *RepositoriesContainer, factory ports.ResourcePoolFactory) ports.ResourcePoolService {
	return usecases.NewResourcePoolService(
		rc.ResourcePoolRepo,
		factory,
	)
}

// createTaskService creates the task definition management service
func createTaskService(rc *RepositoriesContainer) ports.TaskService {
	return usecases.NewTaskService(rc.TaskRepo)
}

// createTaskExecutionService creates the service for task execution management
func createTaskExecutionService(rc *RepositoriesContainer, idGen ports.IDGenerator) ports.TaskExecutionService {
	return usecases.NewTaskExecutionServiceImpl(
		rc.TaskExecutionRepo,
		idGen,
	)
}

// createWorkerDefinitionService creates the service for worker definition management
func createWorkerDefinitionService(rc *RepositoriesContainer, idGen ports.IDGenerator) ports.WorkerDefinitionService {
	return usecases.NewWorkerDefinitionService(
		rc.WorkerDefRepo,
		idGen,
	)
}

// IAM service creation functions

// createAuthService creates the authentication service with necessary dependencies
func createAuthService(
	rc *RepositoriesContainer,
	tokenService ports.TokenService,
	passwordHasher ports.PasswordHasher,
) ports.AuthService {
	return iam.NewAuthenticationService(
		rc.UserRepositoryAdapter,
		rc.TokensRepository,
		tokenService,
		passwordHasher,
	)
}

// createAuthorizationService creates the authorization service for permission checking
func createAuthorizationService(rc *RepositoriesContainer) *iam.AuthorizationService {
	return iam.NewAuthorizationService(
		rc.RoleRepositoryAdapter,
		rc.GroupRepositoryAdapter,
	)
}

// createIdentityService creates the identity management service
func createIdentityService(rc *RepositoriesContainer) *iam.IdentityService {
	return iam.NewIdentityService(rc.UserRepositoryAdapter)
}

// createTokenService creates the JWT token generation and validation service
func createTokenService() ports.TokenService {
	// Get configuration from environment variables or use default values
	accessSecret := getEnv("JWT_ACCESS_SECRET", "default_access_secret")
	refreshSecret := getEnv("JWT_REFRESH_SECRET", "default_refresh_secret")
	accessExpiry := getDurationFromEnv("JWT_ACCESS_EXPIRY", 15*time.Minute)
	refreshExpiry := getDurationFromEnv("JWT_REFRESH_EXPIRY", 7*24*time.Hour)

	return iam.NewJWTTokenService(accessSecret, refreshSecret, accessExpiry, refreshExpiry)
}

// createPasswordHasher creates a password hashing and verification service
func createPasswordHasher() ports.PasswordHasher {
	// Create bcrypt-based password hasher with configurable work factor
	return iam.NewBcryptPasswordHasher(getIntEnv("BCRYPT_WORK_FACTOR", 12))
}
