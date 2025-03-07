package factories

import (
	"dev.rubentxu.devops-platform/orchestrator/config"
	"dev.rubentxu.devops-platform/orchestrator/internal/domain"
	"fmt"
	"log"

	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
)

type WorkerInstanceFactoryImpl struct {
	grpcConfig   config.GrpcConnectionsConfig
	workerConfig interface{}
}

// NewWorkerInstanceFactory recibe la config global (en vez de solo gRPCConfig)
func NewWorkerInstanceFactory(grpcConfig config.GrpcConnectionsConfig) ports.WorkerFactory {
	return &WorkerInstanceFactoryImpl{
		grpcConfig: grpcConfig,
	}
}

func (f *WorkerInstanceFactoryImpl) Create(task domain.TaskExecution, client ports.ResourceIntanceClient) (ports.WorkerInstance, error) {
	log.Printf("[factory] Creating WorkerInstance w/ type=%s", task.WorkerSpec.Type)
	switch task.WorkerSpec.Type {
	case "docker":
		log.Printf("[factory] Creating DockerWorker w/ image=%s", task.WorkerSpec.Image)

		// Tomamos la config específica de Docker

		return NewDockerWorker(task, f.grpcConfig, client)
	case "k8s", "kubernetes":
		log.Printf("[factory] Creating K8sWorker w/ image=%s", task.WorkerSpec.Image)

		return NewK8sWorker(task, f.grpcConfig, client)
	default:
		return nil, fmt.Errorf("unknown InstanceType: %s", task.WorkerSpec.Type)
	}
}
