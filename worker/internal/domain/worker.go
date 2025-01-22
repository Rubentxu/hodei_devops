package domain

import (
	"dev.rubentxu.devops-platform/protos/remote_process"
	"time"
)

type ProcessOutput struct {
	ProcessID string
	Output    string
	IsError   bool
}

type InspectResult struct {
	IsRunning       bool
	State           string
	AdditionalError error
}

type HealthStatus int32

const (
	UNKNOWN  HealthStatus = 0
	RUNNING  HealthStatus = 1
	HEALTHY  HealthStatus = 2
	ERROR    HealthStatus = 3
	STOPPED  HealthStatus = 4
	FINISHED HealthStatus = 5
)

func ConvertProtoProcessStatusToPorts(status remote_process.ProcessStatus) HealthStatus {
	switch status {
	case remote_process.ProcessStatus_UNKNOWN_PROCESS_STATUS:
		return UNKNOWN
	case remote_process.ProcessStatus_RUNNING:
		return RUNNING
	case remote_process.ProcessStatus_HEALTHY:
		return HEALTHY
	case remote_process.ProcessStatus_ERROR:
		return ERROR
	case remote_process.ProcessStatus_STOPPED:
		return STOPPED
	case remote_process.ProcessStatus_FINISHED:
		return FINISHED
	default:
		return UNKNOWN
	}
}

// ProcessHealthStatus representa el estado de un proceso.
type ProcessHealthStatus struct {
	ProcessID string
	Status    HealthStatus
	Message   string
}

type InstanceType string

const (
	DockerInstance     InstanceType = "docker"
	KubernetesInstance InstanceType = "kubernetes"
	VMInstance         InstanceType = "vm"
)

// WorkerSpec describe cómo y dónde se va a ejecutar la tarea.
// Por ejemplo, "docker" vs "k8s", parámetros, etc.
type WorkerSpec struct {
	Type                 InstanceType      `json:"instance_type"`
	Image                string            `json:"image,omitempty"`
	Command              []string          `json:"command,omitempty"`
	Env                  map[string]string `json:"env,omitempty"`
	WorkingDir           string            `json:"working_dir,omitempty"`
	ConnectionParameters map[string]string `json:"connection_parameters,omitempty"`
}

type WorkerConfig struct {
	MaxConcurrentTasks int
	CurrentTasks       int
}

type ScalingEvent struct {
	Timestamp time.Time
	OldLimit  int
	NewLimit  int
	Reason    string
}

type Metrics struct {
	CPUUsage    float64
	MemoryUsage float64
}

type WorkerEndpoint struct {
	WorkerID string
	Address  string
	Port     string
}
