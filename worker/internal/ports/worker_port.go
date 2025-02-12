package ports

import (
	"context"
	"time"

	"dev.rubentxu.devops-platform/worker/internal/domain"
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
	Start(ctx context.Context, outputChan chan<- *domain.ProcessOutput) (*domain.WorkerEndpoint, error)
	Run(ctx context.Context, t domain.TaskExecution, outputChan chan<- *domain.ProcessOutput) error
	Stop(ctx context.Context) (bool, string, error)
	StartMonitoring(ctx context.Context, checkInterval int64, healthChan chan<- *domain.ProcessHealthStatus) error
	GetEndpoint() *domain.WorkerEndpoint
}

type TaskOperation struct {
	//Task       TaskContext
	Task       domain.TaskExecution
	OutputChan chan *domain.ProcessOutput
	Ctx        context.Context
	ErrChan    chan error
}
