package domain

import (
	"log"
	"time"

	"dev.rubentxu.devops-platform/protos/remote_process"
)

type ProcessOutput struct {
	ProcessID string
	Output    string
	IsError   bool
	Type      string
	Status    HealthStatus
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
	PENDING  HealthStatus = 6
	DONE     HealthStatus = 7
)

func (hs HealthStatus) String() string {

	switch hs {
	case PENDING:
		return "PENDING"
	case RUNNING:
		return "RUNNING"
	case HEALTHY:
		return "HEALTHY"
	case ERROR:
		return "ERROR"
	case STOPPED:
		return "STOPPED"
	case FINISHED:
		return "FINISHED"
	case DONE:
		return "DONE"
	default:
		return "UNKNOWN"
	}
}

func ConvertProtoProcessStatusToPorts(status remote_process.ProcessStatus) HealthStatus {
	log.Printf("Convirtiendo status: %v", status)
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
		log.Printf("Status desconocido: %v", status)
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
	Type        InstanceType         `json:"instance_type"`
	Image       string               `json:"image,omitempty"`
	Command     []string             `json:"command,omitempty"`
	Env         map[string]string    `json:"env,omitempty"`
	WorkingDir  string               `json:"working_dir,omitempty"`
	Resources   ResourceRequirements `json:"resources,omitempty"`
	Volumes     []VolumeMount
	Ports       []PortMapping
	Labels      map[string]string
	HealthCheck *HealthCheckConfig
	TemplateID  string
}

type ResourceRequirements struct {
	CPU    float64
	Memory string
}

type VolumeMount struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

type PortMapping struct {
	HostPort      int
	ContainerPort int
	Protocol      string
}

type HealthCheckConfig struct {
	Type     string
	Endpoint string
	Interval time.Duration
	Timeout  time.Duration
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
