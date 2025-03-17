package config

import (
	"context"
	idGenerator "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository"
	executionRepo "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/execution"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/iam"
	resourcepoolRepo "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/resource_pool"
	taskRepo "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/task"
	workerDefRepo "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/worker"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

// RepositoriesContainer holds all data access repositories for dependency injection.
// It provides structured access to all data stores throughout the application.
type RepositoriesContainer struct {
	// Core domain repositories
	ResourcePoolRepo  ports.Repository[*model.ResourcePoolDef, model.AggregateID]
	TaskRepo          ports.Repository[*model.Task, model.AggregateID]
	WorkerDefRepo     ports.Repository[*model.WorkerDefinition, model.AggregateID]
	TaskExecutionRepo ports.Repository[*model.TaskExecution, model.AggregateID]

	// IAM repositories
	UserRepo  ports.Repository[*model.UserAuth, model.AggregateID]
	GroupRepo ports.Repository[*model.Group, model.AggregateID]
	RoleRepo  ports.Repository[*model.Role, model.AggregateID]

	// Repository adapters for specialized operations
	GroupRepositoryAdapter ports.GroupRepository
	UserRepositoryAdapter  ports.UserRepository
	RoleRepositoryAdapter  ports.RoleRepository
	TokensRepository       ports.TokenRepository
}

// InitializeRepositoriesContainer creates and connects all repositories
// and returns them organized in a container ready for injection.
func InitializeRepositoriesContainer() (*RepositoriesContainer, error) {
	// Initialize MongoDB connection
	database, err := connectToMongoDB()
	if err != nil {
		return nil, err
	}

	// Initialize ID Generator
	generator := createIDGenerator()

	// Initialize repositories
	resourcePoolRepo := createResourcePoolMongoDBRepository(database, generator)
	taskRepo := createTaskMongoDBRepository(database, generator)
	workerDefRepo := createWorkerDefMongoDBRepository(database, generator)
	taskExecutionRepo := createTaskExecutionMongoDBRepository(database, generator)
	userRepo := createUserMongoDBRepository(database, generator)
	groupRepo := createGroupMongoDBRepository(database, generator)
	roleRepo := createRoleMongoDBRepository(database, generator)

	// Create specialized repository adapters
	groupRepositoryAdapter := createGroupRepositoryAdapter(groupRepo.(*iam.GroupMongoDBRepository))
	userRepositoryAdapter := createUserRepositoryAdapter(userRepo.(*iam.UserMongoDBRepository))
	roleRepositoryAdapter := createRoleRepositoryAdapter(roleRepo.(*iam.RoleMongoDBRepository))
	tokenRepository := createTokenRepository(database, generator)

	// Return the repositories container with all initialized repositories
	return &RepositoriesContainer{
		ResourcePoolRepo:       resourcePoolRepo,
		TaskRepo:               taskRepo,
		WorkerDefRepo:          workerDefRepo,
		TaskExecutionRepo:      taskExecutionRepo,
		UserRepo:               userRepo,
		GroupRepo:              groupRepo,
		RoleRepo:               roleRepo,
		GroupRepositoryAdapter: groupRepositoryAdapter,
		UserRepositoryAdapter:  userRepositoryAdapter,
		RoleRepositoryAdapter:  roleRepositoryAdapter,
		TokensRepository:       tokenRepository,
	}, nil
}

// connectToMongoDB establishes a connection to MongoDB using environment configuration
// and returns a database instance ready for repository creation.
func connectToMongoDB() (*mongo.Database, error) {
	// Get MongoDB connection parameters from environment
	mongoURI := getEnv("MONGODB_URI", "mongodb://localhost:27017")
	mongoDBName := getEnv("MONGODB_DATABASE", "hodei")

	// Create a context with timeout for the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to MongoDB
	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	// Verify connection with ping
	if err = client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	log.Println("Connected to MongoDB successfully")

	// Return the database instance
	return client.Database(mongoDBName), nil
}

// createIDGenerator initializes an ID generator based on configuration
func createIDGenerator() ports.IDGenerator {
	generatorType := getEnv("ID_GENERATOR", "bson") // Options: "uuid" or "bson"
	return idGenerator.NewIDGenerator(generatorType)
}

// Repository creation functions

// createResourcePoolMongoDBRepository initializes a repository for resource pool definitions
func createResourcePoolMongoDBRepository(db *mongo.Database, generator ports.IDGenerator) ports.Repository[*model.ResourcePoolDef, model.AggregateID] {
	return resourcepoolRepo.NewResourcePoolMongoDBRepository(db, generator)
}

// createTaskMongoDBRepository initializes a repository for task definitions
func createTaskMongoDBRepository(db *mongo.Database, generator ports.IDGenerator) ports.Repository[*model.Task, model.AggregateID] {
	return taskRepo.NewTaskMongoDBRepository(db, generator)
}

// createWorkerDefMongoDBRepository initializes a repository for worker definitions
func createWorkerDefMongoDBRepository(db *mongo.Database, generator ports.IDGenerator) ports.Repository[*model.WorkerDefinition, model.AggregateID] {
	return workerDefRepo.NewWorkerMongoDBRepository(db, generator)
}

// createTaskExecutionMongoDBRepository initializes a repository for task execution records
func createTaskExecutionMongoDBRepository(db *mongo.Database, generator ports.IDGenerator) ports.Repository[*model.TaskExecution, model.AggregateID] {
	return executionRepo.NewTaskExecutionMongoDBRepository(db, generator)
}

// createUserMongoDBRepository initializes a repository for user authentication data
func createUserMongoDBRepository(db *mongo.Database, generator ports.IDGenerator) ports.Repository[*model.UserAuth, model.AggregateID] {
	return iam.NewUserMongoDBRepository(db, generator)
}

// createGroupMongoDBRepository initializes a repository for user groups
func createGroupMongoDBRepository(db *mongo.Database, generator ports.IDGenerator) ports.Repository[*model.Group, model.AggregateID] {
	return iam.NewGroupMongoDBRepository(db, generator)
}

// createRoleMongoDBRepository initializes a repository for roles and permissions
func createRoleMongoDBRepository(db *mongo.Database, generator ports.IDGenerator) ports.Repository[*model.Role, model.AggregateID] {
	return iam.NewRoleMongoDBRepository(db, generator)
}

// Adapter creation functions

// createGroupRepositoryAdapter creates an adapter for the group repository with specialized operations
func createGroupRepositoryAdapter(repo *iam.GroupMongoDBRepository) ports.GroupRepository {
	return iam.NewGroupRepositoryAdapter(repo)
}

// createUserRepositoryAdapter creates an adapter for the user repository with specialized operations
func createUserRepositoryAdapter(repo *iam.UserMongoDBRepository) ports.UserRepository {
	return iam.NewUserRepositoryAdapter(repo)
}

// createRoleRepositoryAdapter creates an adapter for the role repository with specialized operations
func createRoleRepositoryAdapter(repo *iam.RoleMongoDBRepository) ports.RoleRepository {
	return iam.NewRoleRepositoryAdapter(repo)
}

// createTokenRepository creates a repository for token storage and validation
func createTokenRepository(db *mongo.Database, generator ports.IDGenerator) ports.TokenRepository {
	// Use in-memory implementation by default, can be replaced with Redis implementation
	return iam.NewMemoryCachedTokenRepository(db, generator)
}
