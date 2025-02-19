package resources

import (
	"context"
	"dev.rubentxu.devops-platform/worker/config"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"dev.rubentxu.devops-platform/worker/internal/ports"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"io"
)

type DockerResourcePool struct {
	id     string
	client ports.ResourceIntanceClient
}

func (d *DockerResourcePool) GetID() string {
	return d.id
}

func (d *DockerResourcePool) GetResourceInstanceClient() ports.ResourceIntanceClient {
	return d.client
}

func NewDockerResourcePool(config config.DockerConfig, id string) (ports.ResourcePool, error) {

	nativeClient, err := NewDockerClientAdapter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return &DockerResourcePool{client: nativeClient, id: id}, nil
}

func (d *DockerResourcePool) GetStats() (*domain.Stats, error) {
	ctx := context.Background()
	dockerClient := d.client.GetNativeClient().(client.Client)
	containers, err := dockerClient.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	stats := &domain.Stats{
		MemStats:  &domain.MemInfo{},
		DiskStats: &domain.Disk{},
		CpuStats:  &domain.CPUStat{},
		LoadStats: &domain.LoadAvg{},
		TaskCount: len(containers), // Contar contenedores
	}

	var (
		totalCPUUser   uint64
		totalCPUSystem uint64
		totalCPUIdle   uint64
	)

	for _, container := range containers {
		containerStats, err := d.getContainerStats(ctx, container.ID)

		if err != nil {
			continue // Maneja errores individuales; registra si es necesario
		}

		// Sumar las estadísticas de memoria de los contenedores.  Podrías calcular un promedio si lo prefieres.
		stats.MemStats.MemTotal += containerStats.MemoryStats.Limit
		stats.MemStats.MemAvailable += (containerStats.MemoryStats.Limit - containerStats.MemoryStats.Usage) // Asumiendo uso como no disponible
		stats.MemStats.Cached += containerStats.MemoryStats.Stats["cache"]

		// Sumar las estadísticas de CPU. *MUY IMPORTANTE*  Docker reporta uso acumulado.
		totalCPUUser += containerStats.CPUStats.CPUUsage.UsageInUsermode
		totalCPUSystem += containerStats.CPUStats.CPUUsage.UsageInKernelmode
		//docker no da el idle, pero para el calculo total se necesitara el total
		totalCPUIdle += (containerStats.CPUStats.CPUUsage.TotalUsage - containerStats.CPUStats.CPUUsage.UsageInUsermode - containerStats.CPUStats.CPUUsage.UsageInKernelmode)

		//Disco no se puede obtener de stats. Lo obtendremos de la info del contenedor.
		containerInfo, err := dockerClient.ContainerInspect(ctx, container.ID)
		if err != nil {
			continue // o log
		}
		if containerInfo.SizeRootFs != nil {
			stats.DiskStats.All += uint64(*containerInfo.SizeRootFs)
		}
		//Se debe implementar docker en el host para determinar espacio libre y usado
	}

	stats.CpuStats.User = totalCPUUser
	stats.CpuStats.System = totalCPUSystem
	stats.CpuStats.Idle = totalCPUIdle

	return stats, nil
}

func (d *DockerResourcePool) getContainerStats(ctx context.Context, containerID string) (*container.StatsResponse, error) {
	dockerClient := d.client.GetNativeClient().(client.Client)
	statsResp, err := dockerClient.ContainerStats(ctx, containerID, false) // false para no hacer streaming
	if err != nil {
		return nil, err
	}
	defer statsResp.Body.Close()

	body, err := io.ReadAll(statsResp.Body)

	if err != nil {
		return nil, err
	}

	stats := &container.StatsResponse{}

	err = json.Unmarshal(body, stats)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (d *DockerResourcePool) Matches(task domain.Task) bool {
	// Implementación básica.  ¡Ajusta esto a tu WorkerSpec real!
	// Por ejemplo, podrías comprobar si la imagen de Docker existe:
	// ctx := context.Background()
	// _, _, err := d.cli.ImageInspectWithRaw(ctx, task.WorkerSpec.Image)
	// return err == nil

	// Comprobación básica (ejemplo, debe adaptarse a tu WorkerSpec)
	if task.WorkerSpec.Type != "docker" {
		return false
	}

	//Comprobaciones adicionales si es necesario
	return true
}
