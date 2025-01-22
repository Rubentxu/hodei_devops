package domain

import (
	"time"

	"github.com/google/uuid"
)

type Task struct {
	ID         uuid.UUID
	Name       string
	State      State
	WorkerSpec WorkerSpec
	Status     Status
	CreatedAt  time.Time
	UpdatedAt  time.Time
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
