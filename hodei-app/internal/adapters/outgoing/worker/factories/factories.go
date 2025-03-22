package factories

import (
	"dev.rubentxu.hodei-devops/hodei-app/config"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"fmt"
	"log"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
)

type WorkerInstanceFactoryImpl struct {
	grpcConfig   config.Config
	workerConfig interface{}
}

func NewWorkerInstanceFactory(grpcConfig config.Config) ports.WorkerFactory {
	return &WorkerInstanceFactoryImpl{
		grpcConfig: grpcConfig,
	}
}

func (f *WorkerInstanceFactoryImpl) Create(task model.TaskExecution, client ports.ResourceIntanceClient) (ports.WorkerInstance, error) {
	log.Printf("[factory] Creating WorkerInstance w/ type=%s", task.WorkerDef.Spec.Type)
	switch task.WorkerDef.Spec.Type {
	case "docker":
		log.Printf("[factory] Creating DockerWorker w/ image=%s", task.WorkerDef.Spec.Containers[0].Image)

		// Tomamos la config específica de Docker

		return NewDockerWorker(task, f.grpcConfig, client)
	case "k8s", "kubernetes":
		log.Printf("[factory] Creating K8sWorker w/ image=%s", task.WorkerDef.Spec.Containers[0].Image)

		return NewK8sWorker(task, f.grpcConfig, client)
	default:
		return nil, fmt.Errorf("unknown InstanceType: %s", task.WorkerDef.Spec.Type)
	}
}
