package model

import (
	"time"
)

type InstanceType string

const (
	DockerInstance     InstanceType = "docker"
	KubernetesInstance InstanceType = "kubernetes"
	VMInstance         InstanceType = "vm"
)

type WorkerDefinition struct {
	ID       AggregateID
	Metadata Metadata
	Spec     WorkerSpec
	Status   WorkerStatus
}

func (w WorkerDefinition) GetID() AggregateID {
	return w.ID
}

// WorkerSpec describe cómo y dónde se va a ejecutar la tarea.
// Por ejemplo, "docker" vs "k8s", parámetros, etc.
type WorkerSpec struct {
	Type        InstanceType         `json:"instance_type"`
	Image       string               `json:"image,omitempty"`
	Env         map[string]string    `json:"env,omitempty"`
	WorkingDir  string               `json:"working_dir,omitempty"`
	Resources   ResourceRequirements `json:"resources,omitempty"`
	Volumes     []VolumeMount
	Ports       []PortMapping
	Labels      map[string]string
	HealthCheck *HealthCheckConfig
	TemplateID  string
}

type WorkerStatus struct {
	InstanceID string
	Status     HealthStatus
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
