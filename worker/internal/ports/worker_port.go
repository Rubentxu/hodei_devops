package ports

import (
	"context"
	"time"

	"dev.rubentxu.devops-platform/worker/internal/domain"
)

// WorkerPort define el comportamiento que la aplicación espera
// de un "worker": arranque/parada de tareas, actualización, etc.
type WorkerPort interface {
	StartTask(t domain.Task) domain.TaskResult
	StopTask(t domain.Task) domain.TaskResult
	GetTasks() ([]*domain.Task, error)
	AddTask(t domain.Task)
}

// WorkerFactory es una interfaz para crear instancias de workers
type WorkerFactory interface {
	Create(task domain.Task) (WorkerInstance, error)
	GetStopDelay() time.Duration
}

type WorkerInstance interface {
	Start(ctx context.Context, outputChan chan<- *domain.ProcessOutput) (*domain.WorkerEndpoint, error)
	Run(ctx context.Context, t domain.Task, outputChan chan<- *domain.ProcessOutput) error
	Stop(ctx context.Context) (bool, string, error)
	StartMonitoring(ctx context.Context, checkInterval int64, healthChan chan<- *domain.ProcessHealthStatus) error
	GetEndpoint() *domain.WorkerEndpoint
}
