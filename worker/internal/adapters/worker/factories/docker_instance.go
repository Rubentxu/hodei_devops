package factories

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"

	"dev.rubentxu.devops-platform/worker/config"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"github.com/docker/docker/api/types/image"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"dev.rubentxu.devops-platform/worker/internal/adapters/grpc"
	"dev.rubentxu.devops-platform/worker/internal/adapters/security"
	"dev.rubentxu.devops-platform/worker/internal/ports"
)

// DockerWorker implementa WorkerInstance para Docker
type DockerWorker struct {
	task       domain.Task
	endpoint   *domain.WorkerEndpoint
	grpcConfig config.GRPCConfig
	dockerCfg  config.DockerConfig
	client     *dockerclient.Client
}

func NewDockerWorker(task domain.Task, grpcCfg config.GRPCConfig, dockerCfg config.DockerConfig) (ports.WorkerInstance, error) {

	// Ejemplo: usar dockerCfg.Host para crear el cliente
	var opts []dockerclient.Opt
	if dockerCfg.Host != "" {
		opts = append(opts, dockerclient.WithHost(dockerCfg.Host))
	}
	// Podrías añadir TLSVerify si corresponde

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("error creando cliente Docker: %v", err)
	}
	return &DockerWorker{
		task:       task,
		grpcConfig: grpcCfg,
		dockerCfg:  dockerCfg,
		client:     cli,
	}, nil
}

func (d *DockerWorker) Start(ctx context.Context, outputChan chan<- *domain.ProcessOutput) (*domain.WorkerEndpoint, error) {
	log.Printf("Iniciando DockerWorker con spec=%v", d.task.WorkerSpec)

	// Obtener ruta absoluta para el directorio de certificados
	certsPath, err := d.getAbsoluteCertsPath(d.dockerCfg.CertsVolumePath)
	if err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("Error obteniendo ruta de certificados: %v", err))
		return nil, fmt.Errorf("error con ruta de certificados: %v", err)
	}

	// Log para debugging de volúmenes
	log.Printf("Directorio de certificados en host: %s", certsPath)
	if files, err := os.ReadDir(certsPath); err == nil {
		var certFiles []string
		for _, file := range files {
			certFiles = append(certFiles, file.Name())
		}
		log.Printf("Certificados encontrados en host: %v", certFiles)
	}

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

	// Verificar que los certificados existen antes de continuar
	requiredCerts := []string{
		"remote_process-cert.pem",
		"remote_process-key.pem",
		"ca-cert.pem",
		"worker-cert.pem",
		"worker-key.pem",
	}

	for _, cert := range requiredCerts {
		certPath := filepath.Join(certsPath, cert)
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			errMsg := fmt.Sprintf("Certificado requerido no encontrado: %s", certPath)
			d.sendErrorMessage(outputChan, errMsg)
			return nil, fmt.Errorf(errMsg)
		}
		log.Printf("Certificado verificado: %s", cert)
	}

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
		Binds: []string{
			fmt.Sprintf("%s:/certs:ro", certsPath), // Montar certificados como read-only
		},
		NetworkMode: "bridge",
	}

	// Log detallado de la configuración
	log.Printf("Configuración del contenedor:")
	log.Printf("- Volumen de certificados: %s:/certs:ro", certsPath)
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

	// Después de iniciar el contenedor, verificar que los certificados están montados
	if err := d.verifyContainerCerts(ctx, resp.ID); err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("Error verificando certificados en contenedor: %v", err))
		return nil, err
	}

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
func (d *DockerWorker) sendErrorMessage(outputChan chan<- *domain.ProcessOutput, errMsg string) {
	if outputChan == nil {
		return
	}
	outputChan <- &domain.ProcessOutput{
		IsError:   true,
		Output:    errMsg,
		ProcessID: d.task.ID.String(),
	}
}

func (d *DockerWorker) sendLogsMessage(outputChan chan<- *domain.ProcessOutput, msg string) {
	if outputChan == nil {
		return
	}
	outputChan <- &domain.ProcessOutput{
		IsError:   false,
		Output:    msg,
		ProcessID: d.task.ID.String(),
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
func (d *DockerWorker) Run(ctx context.Context, t domain.Task, outputChan chan<- *domain.ProcessOutput) error {
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

// getAbsoluteCertsPath convierte una ruta relativa en absoluta y verifica los certificados
func (d *DockerWorker) getAbsoluteCertsPath(relativePath string) (string, error) {
	var certsPath string

	// Si tenemos una ruta configurada, usarla
	if d.dockerCfg.CertsVolumePath != "" {
		if filepath.IsAbs(d.dockerCfg.CertsVolumePath) {
			certsPath = d.dockerCfg.CertsVolumePath
		} else {
			absPath, err := filepath.Abs(d.dockerCfg.CertsVolumePath)
			if err != nil {
				return "", fmt.Errorf("error convirtiendo ruta configurada a absoluta: %v", err)
			}
			certsPath = absPath
		}
	} else {
		// Si no hay configuración, usar la ruta relativa proporcionada
		absPath, err := filepath.Abs(relativePath)
		if err != nil {
			return "", fmt.Errorf("error convirtiendo a ruta absoluta: %v", err)
		}
		certsPath = absPath
	}

	// Verificar que el directorio existe
	if _, err := os.Stat(certsPath); os.IsNotExist(err) {
		return "", fmt.Errorf("directorio de certificados no existe: %s", certsPath)
	}

	log.Printf("Ruta absoluta de certificados verificada: %s", certsPath)
	return certsPath, nil
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

// Función helper para verificar permisos de archivos
func verifyFilePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error verificando archivo: %v", err)
	}

	mode := info.Mode()
	if mode.Perm()&0444 == 0 { // Verificar si es legible
		return fmt.Errorf("el archivo no tiene permisos de lectura: %s", path)
	}

	return nil
}

// Nuevo método para verificar los certificados en el contenedor
func (d *DockerWorker) verifyContainerCerts(ctx context.Context, containerID string) error {
	// Verificar que el contenedor esté en ejecución
	cont, err := d.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("error inspeccionando contenedor para verificar certificados: %v", err)
	}

	if !cont.State.Running {
		return fmt.Errorf("el contenedor no está en ejecución para verificar certificados")
	}

	// Ejecutar ls en el directorio de certificados dentro del contenedor
	execConfig := types.ExecConfig{
		Cmd:          []string{"ls", "-l", "/certs"},
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
	}

	execResp, err := d.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("error creando exec para verificar certs: %v", err)
	}

	// Adjuntar al exec para obtener la salida
	attach, err := d.client.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return fmt.Errorf("error adjuntando a exec: %v", err)
	}
	defer attach.Close()

	// Leer la salida
	output, err := io.ReadAll(attach.Reader)
	if err != nil {
		return fmt.Errorf("error leyendo salida de exec: %v", err)
	}

	// Verificar que la salida contiene los archivos esperados
	outputStr := string(output)
	log.Printf("Certificados en contenedor:\n%s", outputStr)

	requiredCerts := []string{
		"remote_process-cert.pem",
		"remote_process-key.pem",
		"ca-cert.pem",
	}

	for _, cert := range requiredCerts {
		if !strings.Contains(outputStr, cert) {
			return fmt.Errorf("certificado %s no encontrado en el contenedor", cert)
		}
	}

	return nil
}
