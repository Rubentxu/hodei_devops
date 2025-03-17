package model

import (
	"github.com/google/uuid"
	"time"
)

type TaskExecutionRequest struct {
	TaskID      AggregateID            `json:"task_id"`
	Metadata    Metadata               `json:"metadata"`
	ParamValues map[string]interface{} `json:"param_values"`
}

type TaskExecution struct {
	ID          AggregateID            `json:"id"`
	Metadata    Metadata               `json:"metadata"` // Metadatos de la ejecución
	Task        Task                   `json:"task"`
	Status      ExecutionStatus        `json:"status"`     // Estado de ejecución
	WorkerDef   *WorkerDefinition      `json:"worker_def"` // Worker definition
	ParamValues map[string]interface{} `json:"param_values"`
}

func (t TaskExecution) GetID() AggregateID {
	return t.ID
}

type ExecutionStatus struct {
	ConnectionInfo *ConnectionInfo
	State          TaskState `json:"state"`
	StartTime      time.Time `json:"start_time,omitempty"`
	EndTime        time.Time `json:"end_time,omitempty"`
	Message        string    `json:"message"`
}

type TaskEvent struct {
	ID        uuid.UUID
	State     TaskState
	Timestamp time.Time
	Task      Task
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
