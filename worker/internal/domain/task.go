package domain

import (
	"time"

	"github.com/google/uuid"
)

type Task struct {
	ID    uuid.UUID
	Name  string
	State State
	// Configuración de la instancia donde se ejecutará la tarea
	WorkerInstanceSpec WorkerInstanceSpec

	// En caso de que quieras guardar el tiempo de finalización
	FinishTime time.Time
	Status     Status
}

type Status struct {
	Result   TaskResult
	WorkerID *WorkerID
}

type TaskEvent struct {
	ID        uuid.UUID
	State     State
	Timestamp time.Time
	Task      Task
}

type TaskResult struct {
	Error  error
	Action string
	Result string
}

// Config struct to hold Docker container config
type Config struct {
	// Name of the task, also used as the container name
	Name string
	// AttachStdin boolean which determines if stdin should be attached
	AttachStdin bool
	// AttachStdout boolean which determines if stdout should be attached
	AttachStdout bool
	// AttachStderr boolean which determines if stderr should be attached
	AttachStderr bool
	// ExposedPorts list of ports exposed

}
