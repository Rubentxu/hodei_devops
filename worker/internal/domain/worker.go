package domain

import "dev.rubentxu.devops-platform/protos/remote_process"

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

type ProcessStatus int32

const (
	UNKNOWN  ProcessStatus = 0
	RUNNING  ProcessStatus = 1
	HEALTHY  ProcessStatus = 2
	ERROR    ProcessStatus = 3
	STOPPED  ProcessStatus = 4
	FINISHED ProcessStatus = 5
)

func ConvertProtoProcessStatusToPorts(status remote_process.ProcessStatus) ProcessStatus {
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
	Status    ProcessStatus
	Message   string
}

type WorkerID struct {
	ID      string
	Address string
	Port    string
}

// WorkerInstanceSpec describe cómo y dónde se va a ejecutar la tarea.
// Por ejemplo, "docker" vs "k8s", parámetros, etc.
type WorkerInstanceSpec struct {
	Name string
	// Indica la plataforma: 'docker', 'kubernetes', etc.
	InstanceType               string            `json:"instance_type"`
	Image                      string            `json:"image,omitempty"`
	Command                    []string          `json:"command,omitempty"`
	Env                        map[string]string `json:"env,omitempty"`
	WorkingDir                 string            `json:"working_dir,omitempty"`
	RemoteProcessServerAddress string            `json:"remote_process_server_address"`
}
