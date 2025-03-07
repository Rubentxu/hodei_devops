package ports

import (
	"context"
)

// ProcessExecutor define el contrato para la ejecución de procesos.
type ProcessExecutor interface {
	Start(ctx context.Context, processID string, command []string, env map[string]string, dir string) (<-chan ProcessOutput, error)
	Stop(ctx context.Context, processID string) error
	MonitorHealth(ctx context.Context, processID string, checkInterval int64) (<-chan ProcessHealthStatus, error)
}

// ProcessOutput representa la salida de un proceso.
type ProcessOutput struct {
	ProcessID string
	Output    string
	IsError   bool
	Type      string
	Status    ProcessStatus
}

// Define the ProcessStatus enum
type ProcessStatus int32

const (
	UNKNOWN  ProcessStatus = 0
	RUNNING  ProcessStatus = 1
	HEALTHY  ProcessStatus = 2
	ERROR    ProcessStatus = 3
	STOPPED  ProcessStatus = 4
	FINISHED ProcessStatus = 5
)

// ProcessHealthStatus representa el estado de un proceso.
type ProcessHealthStatus struct {
	ProcessID string
	Status    ProcessStatus
	Message   string
}
