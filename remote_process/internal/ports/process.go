package ports

import (
	"context"
)

// ProcessExecutor define el contrato para la ejecución de procesos.
type ProcessExecutor interface {
	Start(ctx context.Context, processID string, command []string, env map[string]string, dir string) (<-chan ProcessOutput, error)
	Stop(ctx context.Context, processID string) error
	MonitorHealth(ctx context.Context, processID string, checkInterval int64) (<-chan HealthStatus, error)
}

// ProcessOutput representa la salida de un proceso.
type ProcessOutput struct {
	ProcessID string
	Output    string
	IsError   bool
}

// HealthStatus representa el estado de un proceso.
type HealthStatus struct {
	ProcessID string
	IsRunning bool
	Status    string
	IsHealthy bool
}
