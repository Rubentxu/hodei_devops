package ports

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
)

// HodeiAppManager define el comportamiento que la aplicación espera
// de un "worker": arranque/parada de tareas, actualización, etc.
type HodeiAppManager interface {
	StopTask(taskContext TaskContext) error
	AddTask(request model.TaskExecutionRequest, ctx context.Context) (TaskContext, error)
}

// WorkerFactory es una interfaz para crear instancias de workers
type WorkerFactory interface {
	Create(task model.TaskExecution, client ResourceIntanceClient) (WorkerInstance, error)
}

type WorkerInstanceManager interface {
	AddTask(taskContext TaskContext) error
	StopTask(taskContext TaskContext) error
}

type WorkerInstance interface {
	//GetID() model.AggregateID
	//GetName() string
	//GetType() string
	Start(ctx context.Context, templatePath string, outputChan chan<- model.ProcessOutput) (*model.WorkerEndpoint, error)
	Run(ctx context.Context, t model.TaskExecution, outputChan chan<- model.ProcessOutput) error
	Stop(ctx context.Context) (bool, string, error)
	StartMonitoring(ctx context.Context, checkInterval int64, healthChan chan<- *model.ProcessHealthStatus) error
	GetEndpoint() *model.WorkerEndpoint
}

type ResourceIntanceClient interface {
	GetNativeClient() any
	GetConfig() any
}

type TaskContext struct {
	Execution  model.TaskExecution
	OutputChan chan model.ProcessOutput
	StateChan  chan<- model.TaskState // Nuevo canal para el estado
	ErrChan    chan<- error           // Nuevo canal para errores
	Ctx        context.Context
	Client     ResourceIntanceClient
}

type WorkerDefinitionService interface {
	// Operaciones CRUD básicas
	CreateWorkerDefinition(ctx context.Context, workerDef *model.WorkerDefinition) (*model.WorkerDefinition, error)
	GetWorkerDefinition(ctx context.Context, id model.AggregateID) (*model.WorkerDefinition, error)
	UpdateWorkerDefinition(ctx context.Context, updates *model.WorkerDefinition) error
	DeleteWorkerDefinition(ctx context.Context, id model.AggregateID) error

	// Búsqueda y listado
	FindWorkerDefinitions(ctx context.Context, criterio SearchCriteria) (SearchResult[*model.WorkerDefinition], error)
	FindWorkerDefinitionByName(ctx context.Context, name string) (*model.WorkerDefinition, error)
	// Operaciones específicas del dominio
	UpdateWorkerStatus(ctx context.Context, id model.AggregateID, estado model.HealthStatus) error
	AssignTemplate(ctx context.Context, workerID model.AggregateID, templateID string) error

	// Operaciones por lotes
	CreateWorkersBatch(ctx context.Context, workerDefs []*model.WorkerDefinition) ([]*model.WorkerDefinition, error)
	DeleteWorkersBatch(ctx context.Context, ids []model.AggregateID) error
}
