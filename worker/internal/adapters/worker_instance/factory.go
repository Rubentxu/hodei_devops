package worker_instance

import (
	"dev.rubentxu.devops-platform/worker/config"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"fmt"
	"log"

	"dev.rubentxu.devops-platform/worker/internal/adapters/grpc"
	"dev.rubentxu.devops-platform/worker/internal/ports"
)

type WorkerInstanceFactoryImpl struct{}

func NewWorkerInstanceFactory() ports.WorkerInstanceFactory {
	return &WorkerInstanceFactoryImpl{}
}

func (f *WorkerInstanceFactoryImpl) Create(workerConfig domain.WorkerInstanceSpec) (ports.WorkerInstance, error) {
	log.Printf("[factory] Creating WorkerInstance w/ workerConfig=%v", workerConfig)
	if workerConfig.RemoteProcessServerAddress == "" {
		return nil, fmt.Errorf("RemoteProcessServerAddress is required")
	}
	rpcClientConfig := &grpc.ClientConfig{
		ServerAddress: workerConfig.RemoteProcessServerAddress,
		ClientCert:    config.LoadGRPCConfig().ClientCert,
		ClientKey:     config.LoadGRPCConfig().ClientKey,
		CACert:        config.LoadGRPCConfig().CACert,
		JWTToken:      config.LoadGRPCConfig().JWTToken,
	}

	rpcClient, err := grpc.New(rpcClientConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating remote_process_client: %v", err)
	}

	log.Printf("[factory] Created RemoteProcessClient w/ workerConfig=%v", rpcClientConfig)

	switch workerConfig.InstanceType {
	case "docker":
		log.Printf("[factory] Creating DockerWorkerInstance w/ image=%s", workerConfig.Image)
		return NewDockerWorkerInstance(workerConfig, rpcClient), nil
	case "k8s", "kubernetes":
		log.Printf("[factory] Creating K8sWorkerInstance w/ image=%s", workerConfig.Image)
		return NewK8sWorkerInstance(workerConfig, rpcClient), nil
	default:
		return nil, fmt.Errorf("unknown InstanceType: %s", workerConfig.InstanceType)
	}
}
