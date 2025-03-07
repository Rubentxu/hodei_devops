package resources

import (
	"fmt"
	"time"

	"github.com/docker/docker/client"

	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
)

// Config específica para Docker
type DockerResourcesPoolConfig struct {
	Host           string        // e.g., "unix:///var/run/docker.sock", "tcp://localhost:2375" variable de entorno DOCKER_HOST
	NetworkName    string        // Para asegurar que los contenedores están en la misma red
	StopDelay      time.Duration // Tiempo de espera antes de parar el worker
	TLSVerify      bool          // Verificar certificados variables de entorno DOCKER_TLS_VERIFY
	DockerCertPath string        // Ruta a los certificados de Docker variables de entorno DOCKER_CERT_PATH
	Type           string        `json:"type"`
	Name           string        `json:"name"`
	Description    string        `json:"description"`
}

// Implementación de la interfaz ResourcePoolConfig
func (c *DockerResourcesPoolConfig) GetType() string {
	return c.Type
}

func (c *DockerResourcesPoolConfig) GetName() string {
	return c.Name
}

func (c *DockerResourcesPoolConfig) GetDescription() string {
	return c.Description
}

type DockerClientAdapter struct {
	cli    *client.Client
	config DockerResourcesPoolConfig
}

func (d *DockerClientAdapter) GetConfig() any {
	return d.config
}

func (d *DockerClientAdapter) GetNativeClient() any {
	return d.cli
}

// NewDockerClientAdapter crea un DockerClientAdapter y lo retorna como una interfaz ResourceIntanceClient.
func NewDockerClientAdapter(config DockerResourcesPoolConfig) (ports.ResourceIntanceClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation(), client.WithHost(config.Host)) //Usar el host de la configuracion.
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return &DockerClientAdapter{cli: cli, config: config}, nil
}
