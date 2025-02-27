package factories

import (
	"context"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/resources"
	"fmt"
	"io"
	"log"
	"net"
	"path/filepath"
	"strings"
	"time"

	"dev.rubentxu.devops-platform/orchestrator/config"
	"dev.rubentxu.devops-platform/orchestrator/internal/domain"
	"github.com/docker/docker/api/types/image"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/grpc"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/security"
	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
)

// DockerWorker implementa WorkerInstance para Docker
type DockerWorker struct {
	task       domain.TaskExecution
	endpoint   *domain.WorkerEndpoint
	grpcConfig config.GrpcConnectionsConfig
	dockerCfg  resources.DockerResourcesPoolConfig
	client     *dockerclient.Client
}

func (d *DockerWorker) GetID() string {
	return d.task.Name
}

func (d *DockerWorker) GetName() string {
	//TODO implement me
	panic("implement me")
}

func (d *DockerWorker) GetType() string {
	return "docker"
}

func NewDockerWorker(task domain.TaskExecution, grpcCfg config.GrpcConnectionsConfig, resourceClient ports.ResourceIntanceClient) (ports.WorkerInstance, error) {

	cli := resourceClient.GetNativeClient().(*dockerclient.Client)

	return &DockerWorker{
		task:       task,
		grpcConfig: grpcCfg,
		dockerCfg:  resourceClient.GetConfig().(resources.DockerResourcesPoolConfig),
		client:     cli,
		endpoint:   nil,
	}, nil
}

func (d *DockerWorker) Start(ctx context.Context, templatePath string, outputChan chan<- domain.ProcessOutput) (*domain.WorkerEndpoint, error) {
	log.Printf("Iniciando DockerWorker con spec=%v", d.task.WorkerSpec)

	// Environment variables con rutas dentro del contenedor
	baseEnvs := map[string]string{
		"SERVER_CERT_PATH": "/certs/remote_process-cert.pem",
		"SERVER_KEY_PATH":  "/certs/remote_process-key.pem",
		"CA_CERT_PATH":     "/certs/ca-cert.pem",
		"APPLICATION_PORT": "50051",
		"ENV":              d.grpcConfig.Environment,
	}

	// JWT configuration
	jwtSecret := "test_secret_key_for_development_1234567890"
	if d.grpcConfig.JWTSecret != "" {
		jwtSecret = d.grpcConfig.JWTSecret
	}

	// Crear el manejador JWT y generar un nuevo token
	jwtManager := security.NewJWTManager(jwtSecret)
	token, err := jwtManager.GenerateToken("admin")
	if err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("Error generando token JWT: %v", err))
		return nil, fmt.Errorf("error generando token JWT: %v", err)
	}

	// Configurar las variables de entorno JWT
	baseEnvs["JWT_SECRET"] = jwtSecret
	d.grpcConfig.JWTToken = token // Guardar el token generado para uso posterior
	log.Printf("Token JWT generado y configurado para autenticación")

	workerImage := d.task.WorkerSpec.Image
	if workerImage == "" {
		workerImage = "posts_mpv-remote-process:latest"
	}

	containerCfg := &container.Config{
		Image:        workerImage,
		Env:          buildEnvVars(baseEnvs),
		ExposedPorts: nat.PortSet{"50051/tcp": struct{}{}},
		Healthcheck: &container.HealthConfig{
			Test:     []string{"CMD", "/app/grpc_health_check.sh"},
			Interval: 3 * time.Second,
			Timeout:  5 * time.Second,
			Retries:  2,
		},
	}

	// Configuración del host con volumen de certificados
	hostCfg := &container.HostConfig{
		PortBindings: nat.PortMap{
			"50051/tcp": []nat.PortBinding{{
				HostIP:   "0.0.0.0",
				HostPort: "", // Puerto dinámico
			}},
		},
		// TODO: Revisar a futuro: En escenarios Docker in Docker, montar volumenes es complejo por tema de permisos, rutas y volumenes anidados y compartidos
		//Binds: []string{
		//	fmt.Sprintf("%s:/certs:ro", certsPath), // Montar certificados como read-only
		//},
		NetworkMode: "bridge",
	}

	// Log detallado de la configuración
	log.Printf("Configuración del contenedor:")
	log.Printf("- Variables de entorno: %+v", baseEnvs)
	log.Printf("- Configuración de red: %s", hostCfg.NetworkMode)

	// Si hay un working directory en la spec, asegurarse de que sea absoluto
	if d.task.WorkerSpec.WorkingDir != "" {
		absWorkingDir, err := d.toAbsolutePath(d.task.WorkerSpec.WorkingDir)
		if err != nil {
			d.sendLogsMessage(outputChan, fmt.Sprintf("Warning: usando working dir relativo: %v", err))
		} else {
			d.task.WorkerSpec.WorkingDir = absWorkingDir
		}
	}

	// Ajuste de timeout de 60s
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Verificar si existe algún contenedor anterior con el mismo nombre y eliminarlo
	containerName := fmt.Sprintf("task-%s", d.task.Name)
	if err := d.cleanupExistingContainer(ctx, containerName); err != nil {
		d.sendLogsMessage(outputChan, fmt.Sprintf("Warning al limpiar contenedor anterior: %v", err))
	}

	// Verificar si la imagen existe localmente antes de intentar pull
	_, _, err = d.client.ImageInspectWithRaw(ctx, workerImage)
	if err != nil {
		// Si la imagen no existe localmente, intentar pull
		d.sendLogsMessage(outputChan, fmt.Sprintf("Imagen %s no encontrada localmente, intentando pull...", workerImage))
		reader, err := d.client.ImagePull(ctx, workerImage, image.PullOptions{})
		if err != nil {
			// Si falla el pull, enviar mensaje pero continuar (podría existir localmente con otro tag)
			d.sendLogsMessage(outputChan, fmt.Sprintf("No se pudo hacer pull de la imagen %q: %v", workerImage, err))
		} else {
			defer reader.Close()
			// Esperar a que termine el pull
			_, _ = io.Copy(io.Discard, reader)
		}
	}

	log.Printf("Usando imagen %s", workerImage)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Usando imagen %s", workerImage))

	// Crear y arrancar el contenedor
	resp, err := d.client.ContainerCreate(
		ctx,
		containerCfg,
		hostCfg,
		nil,
		nil,
		containerName,
	)
	if err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("Error creando contenedor: %v", err))
		return nil, fmt.Errorf("error creando contenedor Docker: %v", err)
	}

	// Iniciar contenedor
	if err := d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("Error iniciando contenedor: %v", err))
		return nil, fmt.Errorf("error iniciando contenedor Docker: %v", err)
	}
	log.Printf("Contenedor %s iniciado (ID=%s)", containerName, resp.ID)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Contenedor %s iniciado (ID=%s)", containerName, resp.ID))

	// Esperar un momento para que el contenedor esté completamente iniciado
	time.Sleep(2 * time.Second)

	// Inspeccionar contenedor para obtener el puerto
	insp, err := d.client.ContainerInspect(ctx, resp.ID)
	if err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("Error inspeccionando contenedor: %v", err))
		return nil, fmt.Errorf("error inspeccionando contenedor: %v", err)
	}
	hostPort := insp.NetworkSettings.Ports["50051/tcp"][0].HostPort
	log.Printf("Contenedor %s escuchando en puerto %s", containerName, hostPort)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Contenedor %s escuchando en puerto %s", containerName, hostPort))

	// Determinar la dirección del host
	hostAddress := "localhost"
	if d.dockerCfg.Host != "" && d.dockerCfg.Host != "unix:///var/run/docker.sock" {
		// Si tenemos un host Docker remoto, extraer la dirección
		hostAddress = d.dockerCfg.Host
		// Limpiar el prefijo tcp:// si existe
		hostAddress = strings.TrimPrefix(hostAddress, "tcp://")
		// Extraer solo la parte del host si hay puerto
		if host, _, err := net.SplitHostPort(hostAddress); err == nil {
			hostAddress = host
		}
	}
	d.sendLogsMessage(outputChan, fmt.Sprintf("Docker Config: %+v", d.dockerCfg))

	// Guardamos el endpoint
	d.endpoint = &domain.WorkerEndpoint{
		WorkerID: d.task.Name,
		Address:  hostAddress,
		Port:     hostPort,
	}

	log.Printf("Endpoint configurado: %+v", d.endpoint)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Contenedor accesible en %s:%s", hostAddress, hostPort))

	return d.endpoint, nil
}

// sendErrorMessage reenvía un mensaje de error al outputChan si está disponible
func (d *DockerWorker) sendErrorMessage(outputChan chan<- domain.ProcessOutput, errMsg string) {
	if outputChan == nil {
		return
	}
	outputChan <- domain.ProcessOutput{
		IsError:   true,
		Output:    errMsg,
		ProcessID: d.task.ID.String(),
	}
}

func (d *DockerWorker) sendLogsMessage(outputChan chan<- domain.ProcessOutput, msg string) {
	if outputChan == nil {
		return
	}
	outputChan <- domain.ProcessOutput{
		IsError:   false,
		Output:    msg,
		ProcessID: d.task.ID.String(),
		Status:    domain.PENDING,
	}
}

// buildEnvVars convierte un map en un slice de "KEY=VALUE".
func buildEnvVars(env map[string]string) []string {
	var result []string
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

// mergeEnvs mezcla dos mapas, teniendo prioridad el override.
func mergeEnvs(base, override map[string]string) map[string]string {
	newMap := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		newMap[k] = v
	}
	for k, v := range override {
		newMap[k] = v
	}
	return newMap
}

// Run levantará el contenedor Docker y llamará a StartProcess.
func (d *DockerWorker) Run(ctx context.Context, t domain.TaskExecution, outputChan chan<- domain.ProcessOutput) error {
	log.Printf("Iniciando Run para tarea: %s", t.ID)

	grpcClient, err := d.createGRPCClient()
	if err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("Error creando cliente gRPC: %v", err))
		return fmt.Errorf("error creating gRPC client: %w", err)
	}
	defer grpcClient.Close()

	cmds := d.task.WorkerSpec.Command
	if len(cmds) == 0 {
		cmds = []string{"echo", "Hola desde DockerWorker"}
	}

	log.Printf("Ejecutando comando: %v", cmds)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Ejecutando comando: %v", cmds))
	log.Printf("Environment: %v", d.task.WorkerSpec.Env)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Environment: %v", d.task.WorkerSpec.Env))
	log.Printf("WorkingDir: %s", d.task.WorkerSpec.WorkingDir)
	d.sendLogsMessage(outputChan, fmt.Sprintf("WorkingDir: %s", d.task.WorkerSpec.WorkingDir))

	// Llamar al proceso remoto con timeout
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := grpcClient.StartProcess(
		runCtx,
		t.ID.String(),
		cmds,
		d.task.WorkerSpec.Env,
		d.task.WorkerSpec.WorkingDir,
		outputChan,
	); err != nil {
		log.Printf("Error en StartProcess: %v", err)
		return fmt.Errorf("error en StartProcess: %v", err)
	}

	return nil
}

// Stop detiene y limpia el contenedor
func (d *DockerWorker) Stop(ctx context.Context) (bool, string, error) {
	if d.endpoint == nil {
		return true, "No hay contenedor que detener", nil
	}

	containerName := fmt.Sprintf("task-%s", d.task.Name)
	if err := d.cleanupExistingContainer(ctx, containerName); err != nil {
		return false, "", fmt.Errorf("error deteniendo contenedor: %v", err)
	}

	return true, fmt.Sprintf("Contenedor %s detenido y eliminado", containerName), nil
}

// StartMonitoring inicia la monitorización de salud - se mantiene igual
func (d *DockerWorker) StartMonitoring(ctx context.Context, checkInterval int64, healthChan chan<- *domain.ProcessHealthStatus) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	grpcClient, err := d.createGRPCClient()
	if err != nil {
		return fmt.Errorf("error creating gRPC client for stop: %w", err)
	}
	err = grpcClient.MonitorHealth(ctx, d.endpoint.WorkerID, checkInterval, healthChan)
	if err != nil {
		return fmt.Errorf("error abriendo MonitorHealth Docker: %v", err)
	}
	return nil
}

func (d *DockerWorker) createGRPCClient() (*grpc.RPSClient, error) {
	if d.endpoint == nil {
		return nil, fmt.Errorf("endpoint no inicializado")
	}

	// Configuración que coincide con el servidor remote_process
	rpcClientConfig := &grpc.RemoteProcessClientConfig{
		Address:    fmt.Sprintf("%s:%s", d.endpoint.Address, d.endpoint.Port),
		ClientCert: d.grpcConfig.ClientCertPath, // Certificado del cliente
		ClientKey:  d.grpcConfig.ClientKeyPath,  // Llave del cliente
		CACert:     d.grpcConfig.CACertPath,     // Certificado de CA
		AuthToken:  d.grpcConfig.JWTToken,
	}

	log.Printf("Configuración cliente gRPC: %+v", rpcClientConfig)
	return grpc.New(rpcClientConfig)
}

func (w *DockerWorker) GetEndpoint() *domain.WorkerEndpoint {
	return w.endpoint
}

// cleanupExistingContainer elimina un contenedor si existe
func (d *DockerWorker) cleanupExistingContainer(ctx context.Context, containerName string) error {
	containers, err := d.client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("error listando contenedores: %v", err)
	}

	for _, cont := range containers {
		for _, name := range cont.Names {
			// Los nombres de Docker empiezan con /, así que comparamos sin él
			if name == "/"+containerName {
				// Si el contenedor está corriendo, intentar detenerlo primero
				if cont.State == "running" {
					timeout := 10 * time.Second
					timeoutSeconds := int(timeout.Seconds())
					if err := d.client.ContainerStop(ctx, cont.ID, container.StopOptions{Timeout: &timeoutSeconds}); err != nil {
						log.Printf("Error deteniendo contenedor %s: %v", cont.ID, err)
					}
				}
				// Eliminar el contenedor
				if err := d.client.ContainerRemove(ctx, cont.ID, container.RemoveOptions{
					Force:         true,
					RemoveVolumes: true,
				}); err != nil {
					return fmt.Errorf("error eliminando contenedor %s: %v", cont.ID, err)
				}
				log.Printf("Contenedor anterior %s eliminado", containerName)
				return nil
			}
		}
	}
	return nil
}

// Método auxiliar para verificar si un contenedor existe
func (d *DockerWorker) containerExists(ctx context.Context, containerName string) (bool, string) {
	containers, err := d.client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return false, ""
	}

	for _, cont := range containers {
		for _, name := range cont.Names {
			if name == "/"+containerName {
				return true, cont.ID
			}
		}
	}
	return false, ""
}

// toAbsolutePath es un helper genérico para convertir cualquier ruta a absoluta
func (d *DockerWorker) toAbsolutePath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("error convirtiendo a ruta absoluta: %v", err)
	}
	return absPath, nil
}
