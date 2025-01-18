package worker_instance

import (
	"context"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"fmt"
	"github.com/docker/docker/api/types/image"
	"log"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"dev.rubentxu.devops-platform/worker/internal/adapters/grpc"
	"dev.rubentxu.devops-platform/worker/internal/ports"
)

// DockerWorkerInstance implementa WorkerInstance para Docker
type DockerWorkerInstance struct {
	rpcClient    *grpc.Client
	instanceSpec domain.WorkerInstanceSpec
	workerID     *domain.WorkerID
}

func NewDockerWorkerInstance(config domain.WorkerInstanceSpec, rpcClient *grpc.Client) ports.WorkerInstance {
	return &DockerWorkerInstance{
		instanceSpec: config,
		rpcClient:    rpcClient,
	}
}

func (d *DockerWorkerInstance) Start(ctx context.Context) (*domain.WorkerID, error) {
	log.Printf("Iniciando DockerWorkerInstance con spec=%v", d.instanceSpec)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Inicializa el cliente Docker
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error creando cliente Docker: %v", err)
	}
	defer cli.Close()

	// Determinar la imagen
	workerImage := d.instanceSpec.Image
	if workerImage == "" {
		workerImage = "your-default-grpc-workerImage:latest"
	}

	// Pull (opcional, maneja posible error)
	_, err = cli.ImagePull(ctx, workerImage, image.PullOptions{})
	if err != nil {
		log.Printf("No se pudo hacer pull de la imagen %s: %v", workerImage, err)
	}

	// Crear contenedor
	containerName := fmt.Sprintf("task-%s", d.instanceSpec.Name)
	log.Printf("Creando contenedor Docker con nombre %s", containerName)
	resp, err := cli.ContainerCreate(
		ctx,
		&container.Config{
			Image:        workerImage,
			Env:          buildEnvVars(d.instanceSpec.Env),
			ExposedPorts: nat.PortSet{"50051/tcp": {}},
		},
		&container.HostConfig{
			PortBindings: nat.PortMap{
				"50051/tcp": []nat.PortBinding{
					{
						HostIP:   "0.0.0.0",
						HostPort: "",
					},
				},
			},
		},
		nil,
		nil,
		containerName,
	)
	if err != nil {
		return nil, fmt.Errorf("error creando contenedor: %v", err)
	}

	// Iniciar contenedor
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("error iniciando contenedor Docker: %v", err)
	}
	log.Printf("Contenedor %s iniciado (ID=%s)", containerName, resp.ID)

	// Obtener el puerto mapeado
	insp, err := cli.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return nil, fmt.Errorf("error inspeccionando contenedor: %v", err)
	}

	hostPort := ""
	if insp.NetworkSettings != nil {
		if bindings, ok := insp.NetworkSettings.Ports["50051/tcp"]; ok && len(bindings) > 0 {
			hostPort = bindings[0].HostPort
		}
	}
	if hostPort == "" {
		return nil, fmt.Errorf("no se encontró puerto 50051 mapeado")
	}
	d.workerID = &domain.WorkerID{
		ID:      resp.ID,
		Address: d.instanceSpec.RemoteProcessServerAddress,
		Port:    hostPort,
	}
	log.Printf("Definido WorkerID: %v", d.workerID)
	return d.workerID, nil
}

// Run levantará el contenedor Docker y llamará a StartProcess.
func (d *DockerWorkerInstance) Run(ctx context.Context, t domain.Task, outputChan chan<- *domain.ProcessOutput) domain.TaskResult {

	// Conectar al servidor gRPC dentro del contenedor
	grpcAddr := fmt.Sprintf("%s:%s", d.workerID.Address, t.Status.WorkerID.Port)
	newRpcClient, err := grpc.New(&grpc.ClientConfig{
		ServerAddress: grpcAddr,
	})
	if err != nil {
		return domain.TaskResult{Error: fmt.Errorf("error creando cliente gRPC: %v", err)}
	}
	// Podrías almacenar newRpcClient para cerrarlo luego durante Stop, etc.

	// Preparar comandos y entorno
	cmds := d.instanceSpec.Command
	if len(cmds) == 0 {
		cmds = []string{"echo", "Hola desde DockerWorkerInstance"}
	}
	envMap := d.instanceSpec.Env
	if envMap == nil {
		envMap = map[string]string{}
	}

	// Llamar al proceso remoto
	if err := newRpcClient.StartProcess(ctx, t.ID.String(), cmds, envMap, d.instanceSpec.WorkingDir, outputChan); err != nil {
		newRpcClient.Close()
		return domain.TaskResult{Error: fmt.Errorf("error en StartProcess: %v", err)}
	}

	// Retornar el resultado y el canal para que la capa superior decida si
	// consume ese stream directamente (similar a "docker exec")
	taskRes := domain.TaskResult{
		Action: "run",
		Result: "started",
	}

	return taskRes
}

func buildEnvVars(envMap map[string]string) []string {
	var result []string
	for k, v := range envMap {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

// Stop detiene el proceso remoto (gRPC StopProcess) -- puede permanecer igual
func (d *DockerWorkerInstance) Stop() (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	success, msg, err := d.rpcClient.StopProcess(ctx, d.workerID.ID)
	if err != nil {
		return false, msg, fmt.Errorf("error en StopProcess: %v", err)
	}
	log.Printf("Stop Docker process success=%v, msg=%v", success, msg)

	return success, msg, nil
}

// StartMonitoring inicia la monitorización de salud - se mantiene igual
func (d *DockerWorkerInstance) StartMonitoring(ctx context.Context, checkInterval int64, healthChan chan<- *domain.ProcessHealthStatus) error {

	err := d.rpcClient.MonitorHealth(ctx, d.workerID.ID, checkInterval, healthChan)
	if err != nil {
		return fmt.Errorf("error abriendo MonitorHealth Docker: %v", err)
	}

	return nil
}
