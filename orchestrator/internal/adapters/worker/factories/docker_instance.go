package factories

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dev.rubentxu.devops-platform/orchestrator/config"
	"dev.rubentxu.devops-platform/orchestrator/internal/domain"
	_ "github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/grpc"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/security"
	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
)

// DockerWorker implementa WorkerInstance para Docker
type DockerWorker struct {
	task            domain.TaskExecution
	endpoint        *domain.WorkerEndpoint
	grpcConfig      config.GRPCConfig
	dockerCfg       config.DockerConfig
	client          *dockerclient.Client
	composeSpec     *types.Project // Added for Compose-Spec support
	mainServiceName string         // Name of the main service
	mainContainerID string         // Id of the main container
}

func (d *DockerWorker) GetID() string {
	return d.task.ID.String()
}

func (d *DockerWorker) GetName() string {
	return d.task.Name
}

func (d *DockerWorker) GetType() string {
	return "docker"
}

func NewDockerWorker(task domain.TaskExecution, grpcCfg config.GRPCConfig, dockerCfg config.DockerConfig) (ports.WorkerInstance, error) {
	var opts []dockerclient.Opt
	if dockerCfg.Host != "" {
		opts = append(opts, dockerclient.WithHost(dockerCfg.Host))
	}

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating Docker client: %v", err)
	}
	return &DockerWorker{
		task:            task,
		grpcConfig:      grpcCfg,
		dockerCfg:       dockerCfg,
		client:          cli,
		endpoint:        nil,
		composeSpec:     nil,
		mainServiceName: dockerCfg.MainServiceName,
	}, nil
}

func (d *DockerWorker) GetEndpoint() *domain.WorkerEndpoint {
	return d.endpoint
}

// Data structure to pass to the template
type TemplateData struct {
	Task        domain.TaskExecution
	WorkerSpec  domain.WorkerSpec
	Env         string // From config.Config
	JWTSecret   string // From config.GRPCConfig
	ProjectName string
	// ... other global values like cert paths if needed ...
}

func (d *DockerWorker) Start(ctx context.Context, templateString string, outputChan chan<- domain.ProcessOutput) (*domain.WorkerEndpoint, error) {
	log.Printf("Starting DockerWorker with spec=%v", d.task.WorkerSpec)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Starting DockerWorker with spec=%v", d.task.WorkerSpec))

	var err error

	// Check if a Compose file is provided
	if templateString != "" {
		// Example to use
		templateData := TemplateData{
			Task:        d.task,
			WorkerSpec:  d.task.WorkerSpec,
			Env:         d.grpcConfig.Environment, // Config object
			JWTSecret:   d.grpcConfig.JWTSecret,   // Config object
			ProjectName: fmt.Sprintf("task-%s", d.task.Name),
		}
		log.Printf("TemplateData: %+v", templateData)

		// Deploy using Compose-Spec
		d.composeSpec, err = d.loadComposeSpec(ctx, templateString, templateData)
		if err != nil {
			return nil, err
		}

		d.endpoint, err = d.deployWithCompose(ctx, outputChan)
		if err != nil {
			return nil, err
		}

		//copy the files to the main container
		if err := d.copyCertsToMainContainer(outputChan); err != nil {
			return nil, err
		}
	} else {
		// lanza un error si no se proporciona un archivo de plantilla
		return nil, fmt.Errorf("no template file provided")
	}

	return d.endpoint, nil
}

func (d *DockerWorker) loadComposeSpec(ctx context.Context, templateString string, templateData TemplateData) (*types.Project, error) {
	// Definir un mapa con las variables de entorno personalizadas

	customEnv := types.Mapping{
		"MY_IMAGE":     d.task.WorkerSpec.Image,
		"APP_ENV":      templateData.Env,
		"JWT_SECRET":   templateData.JWTSecret,
		"PROJECT_NAME": templateData.ProjectName,
		"JWT_TOKEN":    d.grpcConfig.JWTToken,
	}
	for k, v := range d.task.WorkerSpec.Env {
		customEnv[k] = v
	}
	// Create a ConfigFile object with the template content
	// Crear un objeto ConfigFile con el contenido de la template.
	configFile := types.ConfigFile{
		Filename: "docker-compose.template.yml", // nombre de referencia
		Content:  []byte(templateString),
	}

	// Crear un objeto ConfigDetails que contenga el ConfigFile.
	configDetails := types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{configFile},
		Environment: customEnv,
		WorkingDir:  d.task.WorkerSpec.WorkingDir,
		// Puedes establecer WorkingDir o Environment si lo requieres.
	}
	log.Printf("ConfigDetails: %+v", configDetails)

	// Cargar el proyecto utilizando LoadWithContext (la función recomendada)
	project, err := loader.LoadWithContext(ctx, configDetails)
	if err != nil {
		log.Fatalf("Error cargando el proyecto: %v", err)
	}
	// Obtener la representación YAML final, ya con los placeholders sustituidos.
	rendered, err := project.MarshalYAML()
	if err != nil {
		log.Fatalf("Error serializando el proyecto a YAML: %v", err)
	}

	fmt.Println("YAML final interpolado:")
	fmt.Println(string(rendered))
	return project, nil
}

func (d *DockerWorker) deployWithCompose(ctx context.Context, outputChan chan<- domain.ProcessOutput) (*domain.WorkerEndpoint, error) {
	// Check if composeSpec is loaded
	if d.composeSpec == nil {
		return nil, fmt.Errorf("compose specification not loaded")
	}

	if len(d.composeSpec.Services) == 0 {
		return nil, fmt.Errorf("no services found in compose file")
	}

	// Iterate through networks and create them
	for networkName, networkConfig := range d.composeSpec.Networks {
		_, err := d.client.NetworkInspect(ctx, networkName, network.InspectOptions{})
		if err != nil {
			// If network does not exist, create it
			if dockerclient.IsErrNotFound(err) {
				d.sendLogsMessage(outputChan, fmt.Sprintf("Network %s not found, creating...", networkName))
				_, err := d.client.NetworkCreate(ctx, networkName, network.CreateOptions{
					Driver: networkConfig.Driver,
				})
				if err != nil {
					d.sendLogsMessage(outputChan, fmt.Sprintf("Error creating network %s: %v", networkName, err))
				} else {
					d.sendLogsMessage(outputChan, fmt.Sprintf("Network %s created.", networkName))
				}
			} else {
				return nil, fmt.Errorf("error inspecting network %s: %w", networkName, err)
			}
		}
	}

	var workerEndpoint *domain.WorkerEndpoint
	var err error
	// Iterate through services and create containers
	for name, service := range d.composeSpec.Services {
		workerEndpoint, err = d.deployService(ctx, outputChan, service)
		if err != nil {
			return nil, err
		}
		log.Printf("Worker endpoint and service created for %s: %+v", name, workerEndpoint)
		d.sendLogsMessage(outputChan, fmt.Sprintf("Worker endpoint and service created for %s: %+v", name, workerEndpoint))
		// Set main service name if this is the first service
		if d.mainServiceName == "" {
			d.mainServiceName = name
		}
	}

	return workerEndpoint, nil
}

func (d *DockerWorker) deployService(ctx context.Context, outputChan chan<- domain.ProcessOutput, service types.ServiceConfig) (*domain.WorkerEndpoint, error) {

	workerImage := service.Image
	if workerImage == "" {
		d.sendErrorMessage(outputChan, fmt.Sprintf("No image specified for service %s", service.Name))
		return nil, fmt.Errorf("no image specified for service %s", service.Name)
	}

	d.sendLogsMessage(outputChan, fmt.Sprintf("Using image: %s for service %s", workerImage, service.Name))

	containerName := fmt.Sprintf("task-%s-%s", d.task.Name, service.Name)
	if err := d.cleanupExistingContainer(ctx, containerName); err != nil {
		d.sendLogsMessage(outputChan, fmt.Sprintf("Warning al limpiar contenedor anterior: %v", err))
	}

	_, _, err := d.client.ImageInspectWithRaw(ctx, workerImage)
	if err != nil {
		d.sendLogsMessage(outputChan, fmt.Sprintf("Imagen %s no encontrada localmente, intentando pull...", workerImage))
		reader, err := d.client.ImagePull(ctx, workerImage, image.PullOptions{})
		if err != nil {
			d.sendLogsMessage(outputChan, fmt.Sprintf("No se pudo hacer pull de la imagen %q: %v", workerImage, err))
		} else {
			defer reader.Close()
			if _, err := io.Copy(io.Discard, reader); err != nil {
				d.sendLogsMessage(outputChan, fmt.Sprintf("Error durante el pull de la imagen: %v", err))
			}
		}
	}
	log.Printf("Usando imagen %s", workerImage)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Usando imagen %s", workerImage))

	// Convert service.Environment (MappingWithEquals) to []string
	var envVars []string
	for key, valuePtr := range service.Environment {
		if valuePtr != nil {
			envVars = append(envVars, fmt.Sprintf("%s=%s", key, *valuePtr))
		} else {
			envVars = append(envVars, fmt.Sprintf("%s", key)) // Key without value
		}
	}

	// Convert service.Ports ([]ServicePortConfig) to nat.PortSet
	exposedPorts, err := toExposedPorts(service.Ports)
	// TODO: Cambiar a la siguiente línea para usar el puerto fijo 50051
	//exposedPorts, err := nat.PortSet{
	//	nat.Port("50051/tcp"): struct{}{},
	//}, nil
	if err != nil {
		return nil, err
	}

	// Convert service.Ports ([]ServicePortConfig) to nat.PortMap
	portBindings, err := toPortBindings(service.Ports)
	if err != nil {
		return nil, err
	}

	var networkMode container.NetworkMode
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{},
	}

	if len(service.Networks) > 0 {
		for networkName := range service.Networks {
			if _, ok := d.composeSpec.Networks[networkName]; ok {
				networkMode = container.NetworkMode(networkName)
				networkConfig.EndpointsConfig[networkName] = &network.EndpointSettings{
					NetworkID: networkName,
				}
				break
			} else {
				// Handle the case where the network is not found in d.composeSpec.Networks
				return nil, fmt.Errorf("network '%s' defined for service '%s' not found in global networks configuration", networkName, service.Name)
			}

		}
	}

	containerCreateResp, err := d.client.ContainerCreate(
		ctx,
		&container.Config{
			Image:        workerImage,
			Env:          envVars, // Use the converted environment variables
			ExposedPorts: exposedPorts,
			Healthcheck: &container.HealthConfig{
				Test:     d.dockerCfg.HealthCheck.Test,
				Interval: d.dockerCfg.HealthCheck.Interval,
				Timeout:  d.dockerCfg.HealthCheck.Timeout,
				Retries:  d.dockerCfg.HealthCheck.Retries,
			},
		},
		&container.HostConfig{
			PortBindings: portBindings, // Use the converted port bindings
			NetworkMode:  networkMode,
		},
		networkConfig,
		nil,
		containerName,
	)
	if err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("Error creating container: %v", err))
		return nil, fmt.Errorf("error creating Docker container: %v", err)
	}
	if err := d.client.ContainerStart(ctx, containerCreateResp.ID, container.StartOptions{}); err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("Error starting container: %v", err))
		return nil, fmt.Errorf("error starting Docker container: %v", err)
	}

	// Store the main containerId
	if service.Name == d.mainServiceName {
		d.mainContainerID = containerCreateResp.ID
	}

	log.Printf("Contenedor %s iniciado (ID=%s)", containerName, containerCreateResp.ID)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Contenedor %s iniciado (ID=%s)", containerName, containerCreateResp.ID))

	time.Sleep(2 * time.Second)

	insp, err := d.client.ContainerInspect(ctx, containerCreateResp.ID)
	if err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("Error inspecting container: %v", err))
		d.extractContainerLogs(ctx, containerCreateResp.ID, outputChan)
		return nil, fmt.Errorf("error inspecting container: %v", err)
	}

	// Convertir la estructura a JSON con indentación para una mejor legibilidad
	inspJSON, err := json.MarshalIndent(insp, "", "  ")
	if err != nil {
		log.Printf("Error al formatear inspect: %v", err)
	} else {
		log.Printf("Contenedor %s inspeccionado:\n%s", containerName, string(inspJSON))
	}
	portKey := nat.Port("50051/tcp")
	bindings, ok := insp.NetworkSettings.Ports[portKey]
	if !ok || len(bindings) == 0 {
		d.sendErrorMessage(outputChan, fmt.Sprintf("No se encontraron bindings para el puerto %s", portKey))
		d.extractContainerLogs(ctx, containerCreateResp.ID, outputChan)
		d.sendErrorMessage(outputChan, fmt.Sprintf("Insp json: %s", string(inspJSON)))
		return nil, fmt.Errorf("no se pudieron obtener los bindings para %s", portKey)
	}
	hostPort := bindings[0].HostPort
	log.Printf("Contenedor %s escuchando en puerto %s", containerName, hostPort)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Contenedor %s escuchando en puerto %s", containerName, hostPort))

	if !ok || len(bindings) == 0 || bindings[0].HostPort == "" {
		// Imprimir todas las claves disponibles en el mapa para depurar
		for key, b := range insp.NetworkSettings.Ports {
			log.Printf("Clave de puerto: %s, bindings: %+v", key, b)
		}
		d.extractContainerLogs(ctx, containerCreateResp.ID, outputChan)
		return nil, fmt.Errorf("No port is exposed in binding")
	}

	log.Printf("Contenedor %s escuchando en puerto %s", containerName, hostPort)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Contenedor %s escuchando en puerto %s", containerName, hostPort))

	// Usar el nombre del servicio del worker como dirección, gracias a la red de Docker Compose.
	// TODO: Mirar si es accesible desde el exterior con el nombre del servicio y no localhost.
	hostAddress := d.dockerCfg.WorkerHost
	endpoint := &domain.WorkerEndpoint{
		WorkerID: fmt.Sprintf("%s-%s", d.task.Name, service.Name),
		Address:  hostAddress,
		Port:     hostPort,
	}

	log.Printf("Endpoint configurado: %+v", endpoint)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Contenedor accesible en %s:%s", hostAddress, hostPort))

	return endpoint, nil
}

func (d *DockerWorker) extractContainerLogs(ctx context.Context, containerID string, outputChan chan<- domain.ProcessOutput) {
	logsOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "all",
	}
	out, err := d.client.ContainerLogs(ctx, containerID, logsOptions)
	if err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("error obteniendo logs del contenedor %s: %v", containerID, err))
		return
	}
	defer out.Close()
	data, err := io.ReadAll(out)
	if err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("error leyendo logs del contenedor %s: %v", containerID, err))
		return
	}
	d.sendLogsMessage(outputChan, fmt.Sprintf("[Worker] Logs del contenedor %s:\n%s", containerID, string(data)))
}

// copyCertsToMainContainer copies the certificates from CertsVolumePath to the main service container.
func (d *DockerWorker) copyCertsToMainContainer(outputChan chan<- domain.ProcessOutput) error {
	// Check if composeSpec is loaded and has services
	if d.composeSpec == nil || len(d.composeSpec.Services) == 0 {
		return fmt.Errorf("compose specification not loaded or no services defined")
	}
	if d.mainServiceName == "" {
		return fmt.Errorf("main service name not defined")
	}
	// Get the container ID for the main service
	containerID := d.mainContainerID
	if containerID == "" {
		return fmt.Errorf("container id for main service '%s' not defined", d.mainServiceName)
	}

	// Get all files in CertsVolumePath
	files, err := os.ReadDir(d.dockerCfg.CertsVolumePath)
	if err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("Error reading certs directory: %v", err))
		return fmt.Errorf("error reading certs directory: %w", err)
	}
	log.Printf("Copying certs to container: %s", containerID)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Copying certs to container: %s", containerID))

	// Copy each file to the container
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		localFilePath := filepath.Join(d.dockerCfg.CertsVolumePath, file.Name())
		containerPath := d.dockerCfg.CertsMountPath
		if err := copyFileToContainer(d.client, containerID, containerPath, localFilePath); err != nil {
			d.sendErrorMessage(outputChan, fmt.Sprintf("Error copying %s to container: %v", file.Name(), err))
			return fmt.Errorf("error copying %s to container: %w", file.Name(), err)
		}
		log.Printf("Copied file: %s", file.Name())
		d.sendLogsMessage(outputChan, fmt.Sprintf("Copied file: %s", file.Name()))
	}

	return nil
}

// toExposedPorts converts []types.ServicePortConfig to nat.PortSet
func toExposedPorts(ports []types.ServicePortConfig) (nat.PortSet, error) {
	exposedPorts := nat.PortSet{}
	for _, portConfig := range ports {
		protocol := "tcp"
		if portConfig.Protocol != "" {
			protocol = portConfig.Protocol
		}

		targetPort := fmt.Sprintf("%d/%s", portConfig.Target, protocol)
		exposedPort, err := nat.NewPort(protocol, fmt.Sprintf("%d", portConfig.Target))
		if err != nil {
			return nil, fmt.Errorf("error creating nat.Port: %w", err)
		}
		exposedPorts[exposedPort] = struct{}{}

		log.Printf("Exposing port: %s", targetPort)
	}
	return exposedPorts, nil
}

// toPortBindings converts []types.ServicePortConfig to nat.PortMap
func toPortBindings(ports []types.ServicePortConfig) (nat.PortMap, error) {
	portMap := nat.PortMap{}
	for _, portConfig := range ports {
		protocol := "tcp"
		if portConfig.Protocol != "" {
			protocol = portConfig.Protocol
		}

		containerPort, err := nat.NewPort(protocol, fmt.Sprintf("%d", portConfig.Target))
		if err != nil {
			return nil, fmt.Errorf("error creating nat.Port: %w", err)
		}

		portBinding := nat.PortBinding{}
		if portConfig.Published != "" {
			portBinding.HostPort = fmt.Sprintf("%d", portConfig.Published)
		}
		if portConfig.HostIP != "" {
			portBinding.HostIP = portConfig.HostIP
		}
		log.Printf("Binding port: %s %s", containerPort, portBinding.HostPort)

		portMap[containerPort] = append(portMap[containerPort], portBinding)
	}
	return portMap, nil
}

// copyFileToContainer empaqueta un archivo en tar y lo copia al contenedor.
func copyFileToContainer(cli *dockerclient.Client, containerID, containerPath, localFilePath string) error {
	// Abrir el archivo local.
	file, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("error abriendo archivo %s: %w", localFilePath, err)
	}
	defer file.Close()

	// Crear un buffer para el archivo tar.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("error obteniendo info del archivo %s: %w", localFilePath, err)
	}

	// Crear la cabecera del tar.
	header, err := tar.FileInfoHeader(stat, "")
	if err != nil {
		return fmt.Errorf("error creando cabecera tar: %w", err)
	}
	// Se define el nombre dentro del contenedor.
	header.Name = filepath.Base(localFilePath)

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("error escribiendo cabecera tar: %w", err)
	}
	if _, err := io.Copy(tw, file); err != nil {
		return fmt.Errorf("error copiando contenido a tar: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("error cerrando escritor tar: %w", err)
	}

	options := container.CopyToContainerOptions{
		AllowOverwriteDirWithFile: true,
	}

	if err := cli.CopyToContainer(context.Background(), containerID, containerPath, &buf, options); err != nil {
		return fmt.Errorf("error copiando al contenedor: %w", err)
	}
	return nil
}

func (d *DockerWorker) deployWithDockerConfig(ctx context.Context, outputChan chan<- domain.ProcessOutput) (*domain.WorkerEndpoint, error) {
	jwtSecret := "test_secret_key_for_development_1234567890"
	baseEnvs := map[string]string{
		"SERVER_CERT_PATH": filepath.Join(d.dockerCfg.CertsMountPath, filepath.Base(d.grpcConfig.ServerCertPath)),
		"SERVER_KEY_PATH":  filepath.Join(d.dockerCfg.CertsMountPath, filepath.Base(d.grpcConfig.ServerKeyPath)),
		"CA_CERT_PATH":     filepath.Join(d.dockerCfg.CertsMountPath, filepath.Base(d.grpcConfig.CACertPath)),
		"APPLICATION_PORT": "50051",
		"ENV":              d.grpcConfig.Environment,
		"JWT_SECRET":       jwtSecret,
	}

	if jwtSecret == "" {
		d.sendErrorMessage(outputChan, "JWT_SECRET no configurado")
		return nil, fmt.Errorf("JWT_SECRET no configurado")
	}

	jwtManager := security.NewJWTManager(jwtSecret)
	token, err := jwtManager.GenerateToken("admin")
	if err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("Error generando token JWT: %v", err))
		return nil, fmt.Errorf("error generando token JWT: %v", err)
	}

	d.grpcConfig.JWTToken = token
	log.Printf("Token JWT generado y configurado para autenticación With JWT: %s", token[4:])
	baseEnvs["JWT_SECRET"] = jwtSecret

	workerImage := d.task.WorkerSpec.Image
	if workerImage == "" {
		workerImage = d.dockerCfg.DefaultImage
	}

	containerCfg := &container.Config{
		Image:        workerImage,
		Env:          buildEnvVars(baseEnvs),
		ExposedPorts: nat.PortSet{"50051/tcp": struct{}{}},
		Healthcheck: &container.HealthConfig{
			Test:     d.dockerCfg.HealthCheck.Test,
			Interval: d.dockerCfg.HealthCheck.Interval,
			Timeout:  d.dockerCfg.HealthCheck.Timeout,
			Retries:  d.dockerCfg.HealthCheck.Retries,
		},
	}

	hostCfg := &container.HostConfig{
		PortBindings: nat.PortMap{
			"50051/tcp": []nat.PortBinding{{
				HostIP:   "0.0.0.0",
				HostPort: "", // Puerto dinámico
			}},
		},
		NetworkMode: network.NetworkBridge,
	}

	log.Printf("Configuración del contenedor:")
	log.Printf("- Imagen: %s", workerImage)
	log.Printf("- Variables de entorno: %+v", baseEnvs)
	log.Printf("- Configuración de red: %s", hostCfg.NetworkMode)

	if d.task.WorkerSpec.WorkingDir != "" {
		absWorkingDir, err := d.toAbsolutePath(d.task.WorkerSpec.WorkingDir)
		if err != nil {
			d.sendLogsMessage(outputChan, fmt.Sprintf("Warning: usando working dir relativo: %v", err))
		} else {
			d.task.WorkerSpec.WorkingDir = absWorkingDir
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	containerName := fmt.Sprintf("task-%s", d.task.Name)
	if err := d.cleanupExistingContainer(ctx, containerName); err != nil {
		d.sendLogsMessage(outputChan, fmt.Sprintf("Warning al limpiar contenedor anterior: %v", err))
	}

	_, _, err = d.client.ImageInspectWithRaw(ctx, workerImage)
	if err != nil {
		d.sendLogsMessage(outputChan, fmt.Sprintf("Imagen %s no encontrada localmente, intentando pull...", workerImage))
		reader, err := d.client.ImagePull(ctx, workerImage, image.PullOptions{})
		if err != nil {
			d.sendLogsMessage(outputChan, fmt.Sprintf("No se pudo hacer pull de la imagen %q: %v", workerImage, err))
		} else {
			defer reader.Close()
			if _, err := io.Copy(io.Discard, reader); err != nil {
				d.sendLogsMessage(outputChan, fmt.Sprintf("Error durante el pull de la imagen: %v", err))
			}
		}
	}

	log.Printf("Usando imagen %s", workerImage)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Usando imagen %s", workerImage))

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

	if err := d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		d.sendErrorMessage(outputChan, fmt.Sprintf("Error iniciando contenedor: %v", err))
		return nil, fmt.Errorf("error iniciando contenedor Docker: %v", err)
	}

	log.Printf("Contenedor %s iniciado (ID=%s)", containerName, resp.ID)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Contenedor %s iniciado (ID=%s)", containerName, resp.ID))

	time.Sleep(2 * time.Second)

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

	d.endpoint = &domain.WorkerEndpoint{
		WorkerID: d.task.Name,
		Address:  hostAddress, // Usar el nombre del servicio
		Port:     hostPort,
	}

	log.Printf("Endpoint configurado: %+v", d.endpoint)
	d.sendLogsMessage(outputChan, fmt.Sprintf("Contenedor accesible en %s:%s", hostAddress, hostPort))

	return d.endpoint, nil
}

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

func buildEnvVars(env map[string]string) []string {
	var result []string
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

func (d *DockerWorker) Run(ctx context.Context, t domain.TaskExecution, outputChan chan<- domain.ProcessOutput) error {
	log.Printf("Iniciando Run para tarea: %s", t.ID)

	grpcClient, err := d.createGRPCClient(outputChan) // Pasamos outputChan
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

func (d *DockerWorker) Stop(ctx context.Context) (bool, string, error) {
	if d.composeSpec != nil {
		// If using Compose, remove the entire project
		if err := d.stopComposeProject(ctx); err != nil {
			return false, "", err
		}
		return true, fmt.Sprintf("Project %s stopped and removed", d.composeSpec.Name), nil
	} else {
		// If not using Compose, fall back to the previous behavior of removing a single container
		if d.endpoint == nil {
			return true, "No hay contenedor que detener", nil
		}

		containerName := fmt.Sprintf("task-%s", d.task.Name)
		if err := d.cleanupExistingContainer(ctx, containerName); err != nil {
			return false, "", fmt.Errorf("error deteniendo contenedor: %v", err)
		}
		return true, fmt.Sprintf("Contenedor %s detenido y eliminado", containerName), nil
	}

}

func (d *DockerWorker) stopComposeProject(ctx context.Context) error {
	// Remove containers
	for _, service := range d.composeSpec.Services {
		if err := d.cleanupExistingContainer(ctx, fmt.Sprintf("task-%s-%s", d.task.Name, service.Name)); err != nil {
			return err
		}
	}

	// Remove networks
	for networkName := range d.composeSpec.Networks {
		// List networks with filter
		networks, err := d.client.NetworkList(ctx, network.ListOptions{
			Filters: filters.NewArgs(filters.Arg("name", networkName)),
		})
		if err != nil {
			return err
		}
		// Remove only the network with the same name
		for _, net := range networks {
			if net.Name == networkName {
				err := d.client.NetworkRemove(ctx, net.ID)
				if err != nil {
					return err
				}
			}
		}

	}

	// Remove volumes
	var volumeFilters = filters.NewArgs(filters.Arg("label", fmt.Sprintf("%s=%s", "com.docker.compose.project", d.composeSpec.Name)))
	volumes, err := d.client.VolumeList(ctx, volume.ListOptions{Filters: volumeFilters})
	if err != nil {
		return fmt.Errorf("error listing volumes: %w", err)
	}
	for _, volume := range volumes.Volumes {
		err = d.client.VolumeRemove(ctx, volume.Name, true)
		if err != nil {
			return fmt.Errorf("error removing volume %s: %w", volume.Name, err)
		}
	}

	return nil
}

func (d *DockerWorker) StartMonitoring(ctx context.Context, checkInterval int64, healthChan chan<- *domain.ProcessHealthStatus) error {
	grpcClient, err := d.createGRPCClient(nil) // No necesitamos outputChan aquí
	if err != nil {
		return fmt.Errorf("error creating gRPC client for monitoring: %w", err)
	}
	err = grpcClient.MonitorHealth(ctx, d.endpoint.WorkerID, checkInterval, healthChan)
	if err != nil {
		return fmt.Errorf("error abriendo MonitorHealth Docker: %v", err)
	}
	return nil
}

func (d *DockerWorker) createGRPCClient(outputChan chan<- domain.ProcessOutput) (*grpc.RPSClient, error) {
	if d.endpoint == nil {
		return nil, fmt.Errorf("endpoint no inicializado")
	}

	// d.dockerCfg.WorkerHost en lugar de d.endpoint.Address
	address := fmt.Sprintf("%s:%s", d.dockerCfg.WorkerHost, d.endpoint.Port)
	rpcClientConfig := &grpc.RemoteProcessClientConfig{
		Address:    address, // Usamos la dirección construida
		ClientCert: filepath.Join(d.dockerCfg.CertsMountPath, filepath.Base(d.grpcConfig.ClientCertPath)),
		ClientKey:  filepath.Join(d.dockerCfg.CertsMountPath, filepath.Base(d.grpcConfig.ClientKeyPath)),
		CACert:     filepath.Join(d.dockerCfg.CertsMountPath, filepath.Base(d.grpcConfig.CACertPath)),
		AuthToken:  d.grpcConfig.JWTToken,
	}

	log.Printf("Configuración cliente gRPC: %+v", rpcClientConfig)
	// Solo loguear a outputChan si es proporcionado (para evitar nil pointer)
	if outputChan != nil {
		d.sendLogsMessage(outputChan, fmt.Sprintf("Configuración cliente gRPC: %+v", rpcClientConfig))
	}

	return grpc.New(rpcClientConfig)
}

func (d *DockerWorker) cleanupExistingContainer(ctx context.Context, containerName string) error {
	containers, err := d.client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("error listando contenedores: %v", err)
	}

	for _, cont := range containers {
		for _, name := range cont.Names {
			if name == "/"+containerName {
				if cont.State == "running" {
					timeout := 10 * time.Second
					timeoutSeconds := int(timeout.Seconds())
					if err := d.client.ContainerStop(ctx, cont.ID, container.StopOptions{Timeout: &timeoutSeconds}); err != nil {
						log.Printf("Error deteniendo contenedor %s: %v", cont.ID, err)
					}
				}
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
