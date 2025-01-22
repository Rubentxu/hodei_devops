package factories

import (
	"dev.rubentxu.devops-platform/worker/config"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"fmt"
	"log"

	"dev.rubentxu.devops-platform/worker/internal/ports"
)

type WorkerInstanceFactoryImpl struct {
	appConfig config.Config
}

// NewWorkerInstanceFactory recibe la config global (en vez de solo gRPCConfig)
func NewWorkerInstanceFactory(appCfg config.Config) ports.WorkerFactory {
	return &WorkerInstanceFactoryImpl{
		appConfig: appCfg,
	}
}

func (f *WorkerInstanceFactoryImpl) Create(task domain.Task) (ports.WorkerInstance, error) {
	log.Printf("[factory] Creating WorkerInstance w/ type=%s", task.WorkerSpec.Type)
	switch task.WorkerSpec.Type {
	case "docker":
		log.Printf("[factory] Creating DockerWorker w/ image=%s", task.WorkerSpec.Image)

		// Tomamos la config específica de Docker
		dockerCfg := f.appConfig.Providers.Docker
		// Y también la config gRPC
		grpcCfg := f.appConfig.GRPC
		return NewDockerWorker(task, grpcCfg, dockerCfg)
	case "k8s", "kubernetes":
		log.Printf("[factory] Creating K8sWorker w/ image=%s", task.WorkerSpec.Image)

		// Tomamos la config específica de Kubernetes
		k8sCfg := f.appConfig.Providers.Kubernetes
		// Y también la config gRPC
		grpcCfg := f.appConfig.GRPC
		return NewK8sWorker(task, grpcCfg, k8sCfg)
	default:
		return nil, fmt.Errorf("unknown InstanceType: %s", task.WorkerSpec.Type)
	}
}
