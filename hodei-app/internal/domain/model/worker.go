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

// WorkerDefinition representa la definición completa de un worker
type WorkerDefinition struct {
	ID       AggregateID  `json:"id" validate:"required"`
	Metadata Metadata     `json:"metadata" validate:"required"`
	Spec     WorkerSpec   `json:"spec" validate:"required"`
	Status   WorkerStatus `json:"status"`
}

func (w WorkerDefinition) GetID() AggregateID {
	return w.ID
}

// WorkerSpec describe la configuración del worker
type WorkerSpec struct {
	Type        InstanceType         `json:"instanceType" validate:"required,oneof=docker kubernetes vm"`
	Image       string               `json:"image,omitempty" validate:"required"`
	Env         map[string]string    `json:"env,omitempty" validate:"omitempty,dive,keys,required,endkeys,required"`
	WorkingDir  string               `json:"workingDir,omitempty" validate:"omitempty,dir"`
	Resources   ResourceRequirements `json:"resources" validate:"required"`
	Volumes     []VolumeMount        `json:"volumes,omitempty" validate:"omitempty,dive"`
	Ports       []PortMapping        `json:"ports,omitempty" validate:"omitempty,dive"`
	Labels      map[string]string    `json:"labels,omitempty" validate:"omitempty,dive,keys,required,endkeys,required"`
	HealthCheck *HealthCheckConfig   `json:"healthCheck,omitempty" validate:"omitempty"`
	TemplateID  string               `json:"templateId,omitempty" validate:"omitempty,uuid"`
}

// ResourceRequirements define los recursos necesarios para el worker
type ResourceRequirements struct {
	CPU    float64 `json:"cpu" validate:"required,gt=0"`
	Memory string  `json:"memory" validate:"required,memunit"`
}

// VolumeMount define un punto de montaje para el worker
type VolumeMount struct {
	HostPath      string `json:"hostPath" validate:"required,dir"`
	ContainerPath string `json:"containerPath" validate:"required,dir"`
	ReadOnly      bool   `json:"readOnly"`
}

// PortMapping define un mapeo de puertos para el worker
type PortMapping struct {
	HostPort      int    `json:"hostPort" validate:"required,min=1,max=65535"`
	ContainerPort int    `json:"containerPort" validate:"required,min=1,max=65535"`
	Protocol      string `json:"protocol" validate:"required,oneof=TCP UDP"`
}

// HealthCheckConfig define la configuración del health check
type HealthCheckConfig struct {
	Type     string        `json:"type" validate:"required,oneof=http tcp command"`
	Endpoint string        `json:"endpoint" validate:"required"`
	Interval time.Duration `json:"interval" validate:"required,min=1000000000"`
	Timeout  time.Duration `json:"timeout" validate:"required,min=100000000"`
}

type WorkerStatus struct {
	InstanceID string       `json:"instanceId,omitempty"`
	Status     HealthStatus `json:"status" validate:"oneof=0 1 2 3 4 5 6 7"`
}

// WorkerEndpoint representa información de conexión al worker
type WorkerEndpoint struct {
	Host       string            `json:"host"`
	Port       int               `json:"port"`
	Protocol   string            `json:"protocol"`
	Path       string            `json:"path,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}
