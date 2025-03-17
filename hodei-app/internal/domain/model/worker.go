package model

import (
	"fmt"
	"time"
	"errors"
)

type InstanceType string
type RestartPolicy string
type ImagePullPolicy string
type HostPathType string
type WorkerState string

const (
	DockerInstance            InstanceType    = "docker"
	KubernetesInstance        InstanceType    = "kubernetes"
	VMInstance                InstanceType    = "vm"
	RestartPolicyAlways       RestartPolicy   = "Always"
	RestartPolicyOnFailure    RestartPolicy   = "OnFailure"
	RestartPolicyNever        RestartPolicy   = "Never"
	ImagePullAlways           ImagePullPolicy = "Always"
	ImagePullNever            ImagePullPolicy = "Never"
	ImagePullIfNotPresent     ImagePullPolicy = "IfNotPresent"
	HostPathDirectoryOrCreate HostPathType    = "DirectoryOrCreate"
	HostPathFileOrCreate      HostPathType    = "FileOrCreate"
	HostPathDirectory         HostPathType    = "Directory"
	HostPathFile              HostPathType    = "File"
	HostPathSocket            HostPathType    = "Socket"
	HostPathCharDev           HostPathType    = "CharDevice"
	HostPathBlockDev          HostPathType    = "BlockDevice"
	WorkerStatePending        WorkerState     = "PENDING"
	WorkerStateRunning        WorkerState     = "RUNNING"
	WorkerStateSucceeded      WorkerState     = "SUCCEEDED"
	WorkerStateFailed         WorkerState     = "FAILED"
	WorkerStateStopped        WorkerState     = "STOPPED"
	WorkerStateUnknown        WorkerState     = "UNKNOWN"
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

func (w *WorkerDefinition) Validate() error {
	if w.Metadata.Name == "" {
		return errors.New("name cannot be empty")
	}
	if len(w.Spec.Containers) == 0 || w.Spec.Containers[0].Image == "" {
		return errors.New("image cannot be empty")
	}
	return nil
}

type WorkerSpec struct {
	Type       InstanceType `json:"type" validate:"required,oneof=docker kubernetes vm"`
	Containers []Container  `json:"containers" validate:"required,dive"`
	// +optional
	Volumes []Volume `json:"volumes,omitempty" validate:"omitempty,dive"`
	// +optional
	RestartPolicy RestartPolicy `json:"restartPolicy,omitempty" validate:"omitempty,oneof=Always OnFailure Never"`
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

type Container struct {
	Name            string               `json:"name" validate:"required"`
	Image           string               `json:"image" validate:"required"`
	Command         []string             `json:"command,omitempty"`
	Args            []string             `json:"args,omitempty"`
	Env             []EnvVar             `json:"env,omitempty" validate:"omitempty,dive"`
	Resources       ResourceRequirements `json:"resources" validate:"required"`
	Ports           []PortMapping        `json:"ports,omitempty" validate:"omitempty,dive"`
	VolumeMounts    []VolumeMount        `json:"volumeMounts,omitempty" validate:"omitempty,dive"`
	LivenessProbe   *Probe               `json:"livenessProbe,omitempty"`
	ReadinessProbe  *Probe               `json:"readinessProbe,omitempty"`
	ImagePullPolicy ImagePullPolicy      `json:"imagePullPolicy,omitempty" validate:"omitempty,oneof=Always Never IfNotPresent"`
	WorkingDir      string               `json:"workingDir,omitempty" validate:"omitempty,dir"`
}

type EnvVar struct {
	Name  string `json:"name" validate:"required"`
	Value string `json:"value,omitempty"`
	//EnvVarSource `json:"valueFrom,omitempty"`
}

// ResourceRequirements define los recursos necesarios para el worker
type ResourceRequirements struct {
	CPU    float64 `json:"cpu" validate:"required,gt=0"`
	Memory string  `json:"memory" validate:"required,memunit"`
}

// VolumeMount define un punto de montaje para el worker
type VolumeMount struct {
	Name      string `json:"name" validate:"required"`
	MountPath string `json:"mountPath" validate:"required,dir"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

type Volume struct {
	Name string `json:"name" validate:"required"`
	// Tipo de volumen (e.g., EmptyDir, HostPath, ConfigMap, Secret)
	VolumeSource `json:"volumeSource"`
}

type VolumeSource struct {
	// EmptyDir representa un directorio vacío que se crea al iniciar el worker
	EmptyDir *EmptyDirVolumeSource `json:"emptyDir,omitempty"`
	// HostPath representa un directorio o archivo en el host que se monta en el worker
	HostPath *HostPathVolumeSource `json:"hostPath,omitempty"`
}

type EmptyDirVolumeSource struct {
	// Medium es el tipo de almacenamiento que respalda el directorio vacío (e.g., "Memory")
	Medium string `json:"medium,omitempty"`
	// SizeLimit es el límite de tamaño del directorio vacío
	SizeLimit string `json:"sizeLimit,omitempty"`
}

type HostPathVolumeSource struct {
	// Path es la ruta al directorio o archivo en el host
	Path string `json:"path" validate:"required,dir"`
	// Type es el tipo de hostPath (e.g., DirectoryOrCreate, FileOrCreate)
	Type HostPathType `json:"type,omitempty"`
}

type PortMapping struct {
	ContainerPort int    `json:"containerPort" validate:"required,min=1,max=65535"`
	Protocol      string `json:"protocol" validate:"required,oneof=TCP UDP"`
	// +optional
	HostPort int `json:"hostPort,omitempty" validate:"omitempty,min=1,max=65535"`
	// +optional
	HostIP string `json:"hostIP,omitempty"`
}

type Probe struct {
	// +optional
	Exec *ExecAction `json:"exec,omitempty"`
	// +optional
	HTTPGet *HTTPGetAction `json:"httpGet,omitempty"`
	// +optional
	TCPSocket *TCPSocketAction `json:"tcpSocket,omitempty"`
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty"`
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
}

// ExecAction describe un comando a ejecutar para el health check
type ExecAction struct {
	Command []string `json:"command,omitempty"`
}

// HTTPGetAction describe una petición HTTP para el health check
type HTTPGetAction struct {
	Path        string `json:"path,omitempty"`
	Port        int    `json:"port" validate:"required,min=1,max=65535"`
	Host        string `json:"host,omitempty"`
	Scheme      string `json:"scheme,omitempty" validate:"omitempty,oneof=HTTP HTTPS"`
	HTTPHeaders []HTTPHeader
}

// HTTPHeader define un header para la petición HTTP
type HTTPHeader struct {
	Name  string `json:"name" validate:"required"`
	Value string `json:"value" validate:"required"`
}

// TCPSocketAction describe un socket TCP para el health check
type TCPSocketAction struct {
	Port int    `json:"port" validate:"required,min=1,max=65535"`
	Host string `json:"host,omitempty"`
}

type WorkerStatus struct {
	InstanceID        string            `json:"instanceId,omitempty"`
	State             WorkerState       `json:"state,omitempty"`
	Message           string            `json:"message,omitempty"`
	Reason            string            `json:"reason,omitempty"`
	HostIP            string            `json:"hostIP,omitempty"`
	WorkerIP          string            `json:"workerIP,omitempty"`
	StartTime         *time.Time        `json:"startTime,omitempty"`
	ContainerStatuses []ContainerStatus `json:"containerStatuses,omitempty"`
	QOSClass          string            `json:"qosClass,omitempty"`
}

type ContainerState struct {
	Waiting    *ContainerStateWaiting    `json:"waiting,omitempty"`
	Running    *ContainerStateRunning    `json:"running,omitempty"`
	Terminated *ContainerStateTerminated `json:"terminated,omitempty"`
}

type ContainerStateWaiting struct {
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type ContainerStateRunning struct {
	StartedAt time.Time `json:"startedAt,omitempty"`
}

type ContainerStateTerminated struct {
	ExitCode    int32     `json:"exitCode"`
	Signal      int32     `json:"signal,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	Message     string    `json:"message,omitempty"`
	StartedAt   time.Time `json:"startedAt,omitempty"`
	FinishedAt  time.Time `json:"finishedAt,omitempty"`
	ContainerID string    `json:"containerID,omitempty"`
}

type ContainerStatus struct {
	Name         string         `json:"name"`
	Ready        bool           `json:"ready"`
	RestartCount int32          `json:"restartCount"`
	State        ContainerState `json:"state,omitempty"`
	Image        string         `json:"image"`
	ImageID      string         `json:"imageID"`
	ContainerID  string         `json:"containerID,omitempty"`
}

type ConnectionInfo struct {
	WorkerName    string
	ContainerName string
	Address       string
	Protocol      string
}

func (w *WorkerDefinition) GetConnectionInfo(workerStatus WorkerStatus) ([]ConnectionInfo, error) {
	var connections []ConnectionInfo

	workerIP := workerStatus.HostIP
	if workerIP == "" {
		workerIP = workerStatus.WorkerIP
		if workerIP == "" {
			return nil, fmt.Errorf("no se encontró la IP del worker")
		}
	}

	for _, container := range w.Spec.Containers {
		for _, portMapping := range container.Ports {
			address := fmt.Sprintf("%s:%d", workerIP, portMapping.ContainerPort)
			connections = append(connections, ConnectionInfo{
				WorkerName:    w.Metadata.Name,
				ContainerName: container.Name,
				Address:       address,
				Protocol:      portMapping.Protocol,
			})
		}
	}

	return connections, nil
}
