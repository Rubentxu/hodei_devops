package http

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/incoming/http/handlers"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/incoming/http/middleware"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/incoming/websockets"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/go-playground/validator"
	"github.com/gorilla/mux"
	"net/http"
)

// SetupRouter configura todas las rutas de la API
// SetupRouter configura todas las rutas de la API y WebSockets
func SetupRouter(
	resourcePoolService ports.ResourcePoolService,
	taskService ports.TaskService,
	workerDefinitionService ports.WorkerDefinitionService,
	taskExecutionService ports.TaskExecutionService,
	authService ports.AuthService,
	hodeiApp ports.HodeiAppManager,
) *mux.Router {
	mainRouter := mux.NewRouter()

	// Middleware global
	mainRouter.Use(middleware.LoggingMiddleware)
	mainRouter.Use(middleware.ErrorHandlerMiddleware)

	// API REST Routes
	apiRouter := mainRouter.PathPrefix("/api").Subrouter()

	// ResourcePool handlers
	resourcePoolHandler := handlers.NewResourcePoolHandler(resourcePoolService)
	resources := apiRouter.PathPrefix("/resourcepools").Subrouter()
	resources.HandleFunc("", resourcePoolHandler.CreateResourcePool).Methods(http.MethodPost)
	resources.HandleFunc("", resourcePoolHandler.ListResourcePools).Methods(http.MethodGet)
	resources.HandleFunc("/{id}", resourcePoolHandler.GetResourcePool).Methods(http.MethodGet)
	resources.HandleFunc("/{id}", resourcePoolHandler.UpdateResourcePool).Methods(http.MethodPut)
	resources.HandleFunc("/{id}", resourcePoolHandler.DeleteResourcePool).Methods(http.MethodDelete)
	resources.HandleFunc("/{id}/instance", resourcePoolHandler.CreateResourcePoolInstance).Methods(http.MethodPost)
	resources.HandleFunc("/create-all", resourcePoolHandler.CreateAllResourcePools).Methods(http.MethodPost)
	resources.HandleFunc("/active", resourcePoolHandler.ListActivePools).Methods(http.MethodGet)
	resources.HandleFunc("/active/{id}", resourcePoolHandler.GetActivePool).Methods(http.MethodGet)

	// TaskExecution handlers
	taskExecutionHandler := handlers.NewTaskExecutionHandler(taskExecutionService)
	executions := apiRouter.PathPrefix("/taskexecutions").Subrouter()
	executions.HandleFunc("", taskExecutionHandler.ListTaskExecutions).Methods(http.MethodGet)
	executions.HandleFunc("/{id}", taskExecutionHandler.GetTaskExecution).Methods(http.MethodGet)
	executions.HandleFunc("/metrics", taskExecutionHandler.GetTaskExecutionMetrics).Methods(http.MethodGet)

	// Task handlers
	taskHandler := handlers.NewTaskHandler(taskService)
	tasks := apiRouter.PathPrefix("/tasks").Subrouter()
	tasks.HandleFunc("", taskHandler.CreateTask).Methods(http.MethodPost)
	tasks.HandleFunc("", taskHandler.ListTasks).Methods(http.MethodGet)
	tasks.HandleFunc("/{id}", taskHandler.GetTask).Methods(http.MethodGet)
	tasks.HandleFunc("/{id}", taskHandler.UpdateTask).Methods(http.MethodPut)
	tasks.HandleFunc("/{id}", taskHandler.DeleteTask).Methods(http.MethodDelete)

	// WorkerDefinition handlers
	workerDefinitionHandler := handlers.NewWorkerDefinitionHandler(workerDefinitionService)
	workerDefinitions := apiRouter.PathPrefix("/workerdefinitions").Subrouter()
	workerDefinitions.HandleFunc("", workerDefinitionHandler.CreateWorkerDefinition).Methods(http.MethodPost)
	workerDefinitions.HandleFunc("", workerDefinitionHandler.ListWorkerDefinitions).Methods(http.MethodGet)
	workerDefinitions.HandleFunc("/{id}", workerDefinitionHandler.GetWorkerDefinition).Methods(http.MethodGet)
	workerDefinitions.HandleFunc("/{id}", workerDefinitionHandler.UpdateWorkerDefinition).Methods(http.MethodPut)
	workerDefinitions.HandleFunc("/{id}", workerDefinitionHandler.DeleteWorkerDefinition).Methods(http.MethodDelete)

	// WebSocket handlers
	validate := validator.New()
	wsHandler := websockets.NewWSHandler(hodeiApp, validate)
	mainRouter.HandleFunc("/ws", wsHandler.HandleConnection)

	return mainRouter
}
