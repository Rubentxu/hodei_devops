package ports

import (
	"context"

	"time"

	"dev.rubentxu.devops-platform/orchestrator/internal/domain"
)

// WorkerPort define el comportamiento que la aplicación espera
// de un "worker": arranque/parada de tareas, actualización, etc.
type WorkerPort interface {
	StartTask(t domain.TaskExecution) domain.TaskResult
	StopTask(t domain.TaskExecution) domain.TaskResult
	GetTasks() ([]*domain.TaskExecution, error)
	AddTask(t domain.TaskExecution)
}

// WorkerFactory es una interfaz para crear instancias de workers
type WorkerFactory interface {
	Create(task domain.TaskExecution) (WorkerInstance, error)
	GetStopDelay() time.Duration
}

type WorkerInstance interface {
	GetID() string
	GetName() string
	GetType() string
	Start(ctx context.Context, templatePath string, outputChan chan<- domain.ProcessOutput) (*domain.WorkerEndpoint, error)
	Run(ctx context.Context, t domain.TaskExecution, outputChan chan<- domain.ProcessOutput) error
	Stop(ctx context.Context) (bool, string, error)
	StartMonitoring(ctx context.Context, checkInterval int64, healthChan chan<- *domain.ProcessHealthStatus) error
	GetEndpoint() *domain.WorkerEndpoint
}

type ResourceIntanceClient interface {
	GetNativeClient() any
	GetConfig() any
}

type TaskOperation struct {
	Task       domain.TaskExecution
	OutputChan chan domain.ProcessOutput
	StateChan  chan<- domain.State // Nuevo canal para el estado
	ErrChan    chan<- error        // Nuevo canal para errores
	Ctx        context.Context
	Client     ResourceIntanceClient
}
