package domain

import (
	"time"

	"github.com/google/uuid"
)

type Task struct {
	ID         uuid.UUID
	Name       string
	WorkerSpec WorkerSpec
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type TaskExecution struct {
	ID         uuid.UUID
	Name       string
	TaskID     uuid.UUID // Referencia a la TaskDefinition
	State      State
	StartTime  time.Time
	FinishTime time.Time
	WorkerID   string // ID del contenedor/pod
	HostPorts  []string
	Status     Status
	Error      string    // Para almacenar detalles del error, si lo hay
	CreatedAt  time.Time //Podria no ser necesario, se puede obtener de StartTime
	WorkerSpec WorkerSpec
}

type Status struct {
	Endpoint *WorkerEndpoint
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
