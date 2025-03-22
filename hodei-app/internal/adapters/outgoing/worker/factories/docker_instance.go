package factories

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/incoming/security"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/grpc"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/resource"

	"fmt"
	"io"
	"log"
	"net"
	"path/filepath"
	"strings"
	"time"

	"dev.rubentxu.hodei-devops/hodei-app/config"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"github.com/docker/docker/api/types/image"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
)

// DockerWorker implementa WorkerInstance para Docker
type DockerWorker struct {
	execution      model.TaskExecution
	connectionInfo *model.ConnectionInfo
	config         config.Config
	dockerCfg      resource.DockerResourcesPoolConfig
	client         *dockerclient.Client
	token          string
}

func (d *DockerWorker) GetID() model.AggregateID {
	return d.execution.ID
}

func (d *DockerWorker) GetName() string {
	//TODO implement me
	panic("implement me")
}

func (d *DockerWorker) GetType() string {
	return "docker"
}

func NewDockerWorker(task model.TaskExecution, config config.Config, resourceClient ports.ResourceIntanceClient) (ports.WorkerInstance, error) {

	cli := resourceClient.GetNativeClient().(*dockerclient.Client)
	jwtManager := security.NewJWTManager(config.AccessSecret)
	token, err := jwtManager.GenerateToken("admin")
	if err != nil {
		return nil, fmt.Errorf("error generando token JWT: %v", err)
	}

	return &DockerWorker{
		execution:      task,
		config:         config,
		dockerCfg:      resourceClient.GetConfig().(resource.DockerResourcesPoolConfig),
		client:         cli,
		connectionInfo: nil,
		token:          token,
	}, nil
}

func (d *DockerWorker) Start(ctx context.Context, templatePath string, outputChan chan<- model.ProcessOutput) (*model.ConnectionInfo, error) {
	log.Printf("Iniciando DockerWorker con spec=%v", d.execution.WorkerDef.Spec)

	// Environment variables con rutas dentro del contenedor
	baseEnvs := map[string]string{
		"SERVER_CERT_PATH": "/certs/remote_worker-cert.pem",
		"SERVER_KEY_PATH":  "/certs/remote_worker-key.pem",
		"CA_CERT_PATH":     "/certs/ca-cert.pem",
		"APPLICATION_PORT": "50051",
		"ENV":              d.config.Environment,
	}

	baseEnvs["JWT_SECRET"] = d.token
	log.Printf("Token JWT generado y configurado para autenticación")

	workerImage := d.execution.WorkerDef.Spec.Containers[0].Image
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
	if d.execution.WorkerDef.Spec.Containers[0].WorkingDir != "" {
		absWorkingDir, err := d.toAbsolutePath(d.execution.WorkerDef.Spec.Containers[0].WorkingDir)
		if err != nil {
			d.sendLogsMessage(outputChan, fmt.Sprintf("Warning: usando working dir relativo: %v", err))
		} else {
			d.execution.WorkerDef.Spec.Containers[0].WorkingDir = absWorkingDir
		}
	}

	// Ajuste de timeout de 60s
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Verificar si existe algún contenedor anterior con el mismo nombre y eliminarlo
	containerName := fmt.Sprintf("execution-%s", d.execution.Metadata.Name)
	if err := d.cleanupExistingContainer(ctx, containerName); err != nil {
		d.sendLogsMessage(outputChan, fmt.Sprintf("Warning al limpiar contenedor anterior: %v", err))
	}

	// Verificar si la imagen existe localmente antes de intentar pull
	_, _, err := d.client.ImageInspectWithRaw(ctx, workerImage)
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

	// Guardamos el connectionInfo
	d.connectionInfo = &model.ConnectionInfo{
		WorkerName:    d.execution.Metadata.Name,
		ContainerName: containerName,
		Address:       hostAddress,
		Protocol:      "tcp",
	}

	log.Printf("ConnectionInfo configurado: %+v", d.connectionInfo)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Contenedor accesible en %s:%s", hostAddress, hostPort))

	return d.connectionInfo, nil
}

// sendErrorMessage reenvía un mensaje de error al outputChan si está disponible
func (d *DockerWorker) sendErrorMessage(outputChan chan<- model.ProcessOutput, errMsg string) {
	if outputChan == nil {
		return
	}
	outputChan <- model.ProcessOutput{
		IsError:   true,
		Output:    errMsg,
		ProcessID: d.execution.ID.String(),
	}
}

func (d *DockerWorker) sendLogsMessage(outputChan chan<- model.ProcessOutput, msg string) {
	if outputChan == nil {
		return
	}
	outputChan <- model.ProcessOutput{
		IsError:   false,
		Output:    msg,
		ProcessID: d.execution.ID.String(),
		Status:    model.PENDING,
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
func (d *DockerWorker) Run(ctx context.Context, t model.TaskExecution, outputChan chan<- model.ProcessOutput) error {
	log.Printf("Iniciando Run para tarea: %s", t.ID)

	grpcClient, err := d.createGRPCClient()
	if err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("Error creando cliente gRPC: %v", err))
		return fmt.Errorf("error creating gRPC client: %w", err)
	}
	defer grpcClient.Close()

	cmds := d.execution.Task.Spec.Command
	if len(cmds) == 0 {
		cmds = []string{"echo", "Hola desde DockerWorker"}
	}

	log.Printf("Ejecutando comando: %v", cmds)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Ejecutando comando: %v", cmds))
	log.Printf("Environment: %v", d.execution.WorkerDef.Spec.Containers[0].Env)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Environment: %v", d.execution.WorkerDef.Spec.Containers[0].Env))
	log.Printf("WorkingDir: %s", d.execution.WorkerDef.Spec.Containers[0].WorkingDir)
	d.sendLogsMessage(outputChan, fmt.Sprintf("WorkingDir: %s", d.execution.WorkerDef.Spec.Containers[0].WorkingDir))

	// Llamar al proceso remoto con timeout
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := grpcClient.StartProcess(
		runCtx,
		t.ID.String(),
		cmds,
		d.execution.WorkerDef.Spec.Containers[0].Env,
		d.execution.WorkerDef.Spec.Containers[0].WorkingDir,
		outputChan,
	); err != nil {
		log.Printf("Error en StartProcess: %v", err)
		return fmt.Errorf("error en StartProcess: %v", err)
	}

	return nil
}

// Stop detiene y limpia el contenedor
func (d *DockerWorker) Stop(ctx context.Context) (bool, string, error) {
	if d.connectionInfo == nil {
		return true, "No hay contenedor que detener", nil
	}

	containerName := fmt.Sprintf("execution-%s", d.execution.Metadata.Name)
	if err := d.cleanupExistingContainer(ctx, containerName); err != nil {
		return false, "", fmt.Errorf("error deteniendo contenedor: %v", err)
	}

	return true, fmt.Sprintf("Contenedor %s detenido y eliminado", containerName), nil
}

// StartMonitoring inicia la monitorización de salud - se mantiene igual
func (d *DockerWorker) StartMonitoring(ctx context.Context, checkInterval int64, healthChan chan<- *model.ProcessHealthStatus) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	grpcClient, err := d.createGRPCClient()
	if err != nil {
		return fmt.Errorf("error creating gRPC client for stop: %w", err)
	}
	err = grpcClient.MonitorHealth(ctx, d.connectionInfo.WorkerName, checkInterval, healthChan)
	if err != nil {
		return fmt.Errorf("error abriendo MonitorHealth Docker: %v", err)
	}
	return nil
}

func (d *DockerWorker) createGRPCClient() (*grpc.RPSClient, error) {
	if d.connectionInfo == nil {
		return nil, fmt.Errorf("connectionInfo no inicializado")
	}

	// Configuración que coincide con el servidor remote_worker
	rpcClientConfig := &grpc.RemoteProcessClientConfig{
		Address:    d.connectionInfo.Address,
		ClientCert: d.config.ClientCertPath, // Certificado del cliente
		ClientKey:  d.config.ClientKeyPath,  // Llave del cliente
		CACert:     d.config.CACertPath,     // Certificado de CA
		AuthToken:  d.token,                 // Token JWT
	}

	log.Printf("Configuración cliente gRPC: %+v", rpcClientConfig)
	return grpc.New(rpcClientConfig)
}

func (w *DockerWorker) GetEndpoint() *model.ConnectionInfo {
	return w.connectionInfo
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
