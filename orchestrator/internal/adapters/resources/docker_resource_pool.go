package resources

import (
	"context"
	"dev.rubentxu.devops-platform/orchestrator/config"
	"dev.rubentxu.devops-platform/orchestrator/internal/domain"
	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"io"
	"log"
	"sync"
)

type DockerResourcePool struct {
	id           string
	client       ports.ResourceIntanceClient
	lastCPUStats map[string]*container.CPUStats
	mu           sync.Mutex
	hostInfo     *system.Info // Cache the Docker host info
	hostInfoErr  error        // Cache the error for fetching host info
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

	// Fetch Docker host info immediately
	dockerClient := nativeClient.GetNativeClient().(*client.Client)
	ctx := context.Background()
	hostInfo, err := dockerClient.Info(ctx)
	if err != nil {
		log.Printf("Error getting Docker host info: %v", err)
	}

	return &DockerResourcePool{
		id:           id,
		client:       nativeClient,
		lastCPUStats: make(map[string]*container.CPUStats),
		hostInfo:     &hostInfo, // Store Docker host info in the struct
		hostInfoErr:  err,       // Store the error
	}, nil
}

func (d *DockerResourcePool) GetStats() (*domain.Stats, error) {
	log.Println("Getting stats from Docker resource pool")
	ctx := context.Background()
	dockerClient := d.client.GetNativeClient().(*client.Client)

	// --- Check for cached host info and error ---
	if d.hostInfoErr != nil {
		log.Printf("Error al obtener información del host Docker: %v", d.hostInfoErr)
		// Continuar sin la información del host, usando valores por defecto.
	}

	containers, err := dockerClient.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	stats := &domain.Stats{
		MemStats:  &domain.MemInfo{},
		DiskStats: &domain.Disk{},
		CpuStats:  &domain.CPUStat{},
		LoadStats: &domain.LoadAvg{},
		TaskCount: len(containers),
		HostInfo:  make(map[string]interface{}), // Inicializa el mapa
	}

	var (
		totalCPUUser   uint64
		totalCPUSystem uint64
		totalCPUIdle   uint64
		totalMemUsage  uint64
		totalMemCache  uint64
		totalDiskUsage uint64

		totalMemLimit uint64
	)

	// --- Parallelized Stats Collection ---
	var wg sync.WaitGroup
	containerStatsChan := make(chan containerStatsResult, len(containers))

	for _, container := range containers {
		wg.Add(1)
		go func(containerID string) {
			defer wg.Done()
			containerStats, err := d.getContainerStats(ctx, containerID)
			containerInfo, errInfo := dockerClient.ContainerInspect(ctx, containerID)
			containerStatsChan <- containerStatsResult{containerID: containerID, stats: containerStats, info: containerInfo, err: err, errInfo: errInfo}
		}(container.ID)
	}
	wg.Wait()
	close(containerStatsChan)

	// --- Process Results ---
	for result := range containerStatsChan {
		if result.err != nil {
			log.Printf("Error getting stats for container %s: %v", result.containerID, result.err)
			continue
		}
		if result.errInfo != nil {
			log.Printf("Error getting info for container %s: %v", result.containerID, result.errInfo)
			continue
		}

		// --- Memory ---
		totalMemLimit += result.stats.MemoryStats.Limit
		totalMemUsage += result.stats.MemoryStats.Usage
		totalMemCache += result.stats.MemoryStats.Stats["cache"]
		log.Printf("Container %s: Memory Usage %d/%d (Cache: %d)\n", result.containerID, result.stats.MemoryStats.Usage, result.stats.MemoryStats.Limit, result.stats.MemoryStats.Stats["cache"])

		// --- CPU Usage Calculation ---
		cpuUser, cpuSystem, cpuIdle := d.calculateCPUUsage(result.containerID, &result.stats.CPUStats)
		totalCPUUser += cpuUser
		totalCPUSystem += cpuSystem
		totalCPUIdle += cpuIdle

		// --- Disk Usage ---
		if result.info.SizeRootFs != nil {
			totalDiskUsage += uint64(*result.info.SizeRootFs)
		}
		log.Printf("Container %s: RootFS Size %d\n", result.containerID, result.info.SizeRootFs)
	}

	// --- Assign Container Statistics ---
	stats.MemStats.MemTotal = totalMemLimit
	stats.MemStats.MemAvailable = totalMemLimit - totalMemUsage // Simplification; improve with cache info
	stats.MemStats.Cached = totalMemCache

	stats.CpuStats.User = totalCPUUser
	stats.CpuStats.System = totalCPUSystem
	stats.CpuStats.Idle = totalCPUIdle

	stats.DiskStats.All = totalDiskUsage

	// --- Host Resource Utilization Calculation (Limited to info from Docker API) ---
	if d.hostInfo != nil {
		// Usar 0 como valor por defecto si no se puede obtener la información.
		numCPU := d.hostInfo.NCPU
		if numCPU <= 0 {
			numCPU = 1 // Valor por defecto si no hay CPUs
		}
		stats.HostInfo["TotalCores"] = numCPU

		memTotal := d.hostInfo.MemTotal
		if memTotal <= 0 {
			memTotal = 1024 * 1024 * 1024 // 1GB como valor por defecto
		}
		stats.HostInfo["MemTotal"] = memTotal
		stats.HostInfo["OSType"] = d.hostInfo.OSType
		stats.HostInfo["Architecture"] = d.hostInfo.Architecture

		memoryUsagePercent := float64(totalMemUsage) / float64(memTotal) * 100
		stats.HostInfo["MemoryUsagePercent"] = memoryUsagePercent

		log.Printf("Memory Usage: %.2f%% (Containers) of total host memory \n", memoryUsagePercent)

		// No disk stats available directly from Docker API.  Consider alternatives.
	} else {
		log.Println("No host info available from Docker API.")
		// Asignar valores por defecto si no se puede obtener la información del host.
		stats.HostInfo["TotalCores"] = 1
		stats.HostInfo["MemTotal"] = uint64(1024 * 1024 * 1024) // 1GB
		stats.HostInfo["MemoryUsagePercent"] = 0.0
	}

	log.Printf("Total CPU (User, System, Idle): %d %d %d\n", totalCPUUser, totalCPUSystem, totalCPUIdle)
	log.Printf("Total Memory (Limit, Usage, Cache): %d %d %d\n", totalMemLimit, totalMemUsage, totalMemCache)
	log.Printf("Total Disk (All): %d\n", totalDiskUsage)

	return stats, nil
}

type containerStatsResult struct {
	containerID string
	stats       *container.StatsResponse
	info        types.ContainerJSON
	err         error
	errInfo     error
}

func (d *DockerResourcePool) getContainerStats(ctx context.Context, containerID string) (*container.StatsResponse, error) {
	dockerClient := d.client.GetNativeClient().(*client.Client)
	statsResp, err := dockerClient.ContainerStats(ctx, containerID, false)
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

func (d *DockerResourcePool) calculateCPUUsage(containerID string, currentStats *container.CPUStats) (uint64, uint64, uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	previousStats, ok := d.lastCPUStats[containerID]
	d.lastCPUStats[containerID] = currentStats

	if !ok || previousStats == nil {
		// No previous stats available, return zero usage.
		return 0, 0, 0
	}

	cpuDelta := float64(currentStats.CPUUsage.TotalUsage - previousStats.CPUUsage.TotalUsage)
	systemDelta := float64(currentStats.SystemUsage - previousStats.SystemUsage)

	if systemDelta > 0.0 {
		cpuPercent := (cpuDelta / systemDelta) * float64(len(currentStats.CPUUsage.PercpuUsage)) * 100.0
		// Convertir el porcentaje a unidades de uso (puedes ajustar esto según tus necesidades)
		cpuUser := uint64(cpuPercent * float64(currentStats.CPUUsage.UsageInUsermode) / float64(currentStats.CPUUsage.TotalUsage))
		cpuSystem := uint64(cpuPercent * float64(currentStats.CPUUsage.UsageInKernelmode) / float64(currentStats.CPUUsage.TotalUsage))
		cpuIdle := uint64(cpuPercent * float64(uint64(currentStats.OnlineCPUs)-currentStats.CPUUsage.UsageInUsermode-currentStats.CPUUsage.UsageInKernelmode) / float64(currentStats.CPUUsage.TotalUsage))

		return cpuUser, cpuSystem, cpuIdle
	}

	return 0, 0, 0
}

func (d *DockerResourcePool) Matches(task domain.Task) bool {
	if task.WorkerSpec.Type != "docker" {
		return false
	}
	return true
}
