package model

import (
	"dev.rubentxu.hodei-devops/protos/remote_worker"
	"log"
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

func ConvertProtoProcessStatusToPorts(status remote_worker.ProcessStatus) HealthStatus {
	log.Printf("Convirtiendo status: %v", status)
	switch status {
	case remote_worker.ProcessStatus_UNKNOWN_PROCESS_STATUS:
		return UNKNOWN
	case remote_worker.ProcessStatus_RUNNING:
		return RUNNING
	case remote_worker.ProcessStatus_HEALTHY:
		return HEALTHY
	case remote_worker.ProcessStatus_ERROR:
		return ERROR
	case remote_worker.ProcessStatus_STOPPED:
		return STOPPED
	case remote_worker.ProcessStatus_FINISHED:
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
