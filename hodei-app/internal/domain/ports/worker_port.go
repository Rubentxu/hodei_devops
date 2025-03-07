package ports

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
)

// WorkerPort define el comportamiento que la aplicación espera
// de un "worker": arranque/parada de tareas, actualización, etc.
type WorkerPort interface {
	StartTask(t model.TaskExecution) model.TaskResult
	StopTask(t model.TaskExecution) model.TaskResult
	GetTasks() ([]*model.TaskExecution, error)
	AddTask(t model.TaskExecution)
}

// WorkerFactory es una interfaz para crear instancias de workers
type WorkerFactory interface {
	Create(task model.TaskExecution, client ResourceIntanceClient) (WorkerInstance, error)
}

type WorkerInstance interface {
	GetID() string
	GetName() string
	GetType() string
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

type TaskOperation struct {
	Task       model.TaskExecution
	OutputChan chan model.ProcessOutput
	StateChan  chan<- model.State // Nuevo canal para el estado
	ErrChan    chan<- error       // Nuevo canal para errores
	Ctx        context.Context
	Client     ResourceIntanceClient
}
