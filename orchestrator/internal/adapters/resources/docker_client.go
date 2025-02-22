package resources

import (
	"fmt"

	"github.com/docker/docker/client"

	"dev.rubentxu.devops-platform/orchestrator/config"
	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
)

type DockerClientAdapter struct {
	cli    *client.Client
	config config.DockerConfig
}

func (d *DockerClientAdapter) GetConfig() any {
	return d.config
}

func (d *DockerClientAdapter) GetNativeClient() any {
	return d.cli
}

// NewDockerClientAdapter crea un DockerClientAdapter y lo retorna como una interfaz ResourceIntanceClient.
func NewDockerClientAdapter(config config.DockerConfig) (ports.ResourceIntanceClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation(), client.WithHost(config.Host)) //Usar el host de la configuracion.
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return &DockerClientAdapter{cli: cli, config: config}, nil
}
