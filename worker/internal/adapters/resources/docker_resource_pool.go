package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"dev.rubentxu.devops-platform/worker/internal/domain"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type DockerProvider struct {
	BaseProvider
	client *client.Client
	pools  map[string]*DockerPool
	mutex  sync.RWMutex
}

// dockerStats represents the relevant parts of the stats JSON.
type dockerStats struct {
	CPUStats    cpuStats    `json:"cpu_stats"`
	PreCPUStats cpuStats    `json:"precpu_stats"` //  Previous CPU stats for delta calculation.
	MemoryStats memoryStats `json:"memory_stats"`
}

type cpuStats struct {
	CPUUsage struct {
		TotalUsage        uint64   `json:"total_usage"`
		PercpuUsage       []uint64 `json:"percpu_usage"` //  Important for scaling CPU usage to host cores.
		UsageInKernelmode uint64   `json:"usage_in_kernelmode"`
		UsageInUsermode   uint64   `json:"usage_in_usermode"`
	} `json:"cpu_usage"`
	SystemUsage    uint64   `json:"system_cpu_usage"` // System-wide CPU usage.  Essential for delta calculation.
	OnlineCPUs     int      `json:"online_cpus"`      // Number of online CPUs. May be 0; use length of PercpuUsage if so.
	ThrottlingData struct { //  If you need throttling information.
		Periods          uint64 `json:"periods"`
		ThrottledPeriods uint64 `json:"throttled_periods"`
		ThrottledTime    uint64 `json:"throttled_time"`
	} `json:"throttling_data"`
}

type memoryStats struct {
	Usage    uint64            `json:"usage"`
	MaxUsage uint64            `json:"max_usage"` // Peak memory usage.
	Stats    map[string]uint64 `json:"stats"`     // Detailed memory stats (cache, rss, etc.)
	Limit    uint64            `json:"limit"`     // The actual memory limit.  CRITICAL.
	Failcnt  uint64            `json:"failcnt"`   // Number of times memory usage hit the limit
}

// ************** Docker Provider Implementation **************
func (d *DockerProvider) DeletePool(ctx context.Context, poolID string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	pool, exists := d.pools[poolID]
	if !exists {
		return fmt.Errorf("pool %s not found", poolID)
	}

	// Stop and remove all containers associated with the pool
	containers, err := d.client.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "label",
			Value: pool.Label,
		}),
	})
	if err != nil {
		return err
	}

	for _, c := range containers {
		if err := d.client.ContainerStop(ctx, c.ID, container.StopOptions{}); err != nil {
			return err
		}
		if err := d.client.ContainerRemove(ctx, c.ID, container.RemoveOptions{}); err != nil {
			return err
		}
	}

	// Remove the pool from the provider's map
	delete(d.pools, poolID)
	return nil
}

func (d *DockerProvider) GetResourceUsage(ctx context.Context, poolID string, resourceType domain.ResourceType) (domain.ResourceUsage, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	pool, exists := d.pools[poolID]
	if !exists {
		return domain.ResourceUsage{}, fmt.Errorf("pool %s not found", poolID)
	}

	// Get all containers in the pool
	containers, err := d.client.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "label",
			Value: pool.Label,
		}),
	})
	if err != nil {
		return domain.ResourceUsage{}, err
	}

	// Aggregate metrics for the specific resource type
	var totalUsed, totalMax float64

	for _, c := range containers {
		stats, err := d.client.ContainerStats(ctx, c.ID, false) //  false = don't stream.
		if err != nil {
			fmt.Printf("Error getting stats for container %s: %v\n", c.ID, err)
			continue // Or handle error appropriately
		}
		defer stats.Body.Close()

		metrics, err := parseDockerStats(stats.Body)
		if err != nil {
			fmt.Printf("Error parsing stats for container %s: %v\n", c.ID, err)
			continue
		}

		used, ok := metrics[resourceType]
		if !ok {
			fmt.Printf("Resource type %s not found in metrics for container %s\n", resourceType, c.ID)
			continue // Or handle appropriately
		}
		totalUsed += used

		// Determine "Max" based on resource type.
		switch resourceType {
		case domain.CPU:
			if cpuLimit, ok := pool.Constraints[domain.CPU]; ok {
				maxCPU, _ := strconv.ParseFloat(cpuLimit, 64)

				// Get number of CPUs from stats.  Use PercpuUsage length.
				statBytes, _ := io.ReadAll(stats.Body) //read body for second time, bad practice,
				stats.Body.Close()                     // close body after io.ReadAll
				var statJSON container.StatsResponse   // create statJSON for unmarshal purposes
				json.Unmarshal(statBytes, &statJSON)   //Unmarshal

				numCPUs := len(statJSON.CPUStats.CPUUsage.PercpuUsage)
				if numCPUs > 0 {
					totalMax += maxCPU * float64(numCPUs)
				} else {
					// Fallback:  If PercpuUsage is empty, assume 1 CPU (unlikely but possible).
					totalMax += maxCPU
				}
			}
		case domain.Memory:
			// Get memory limit directly from the stats.  This is the *correct* way.
			dec := json.NewDecoder(stats.Body) // reset decoder
			var statsJSON container.StatsResponse
			if err := dec.Decode(&statsJSON); err != nil {
				fmt.Println("Error al decodificar JSON:", err)
			}
			totalMax += float64(statsJSON.MemoryStats.Limit) // Use the actual limit!

		}
	}

	// Calculate available based on constraints and used resources
	constraint, hasConstraint := pool.Constraints[resourceType]
	var max, available float64
	if hasConstraint {
		max = parseConstraint(constraint, resourceType)
		available = max - totalUsed

	} else {
		max = totalMax
		available = max - totalUsed
	}

	return domain.ResourceUsage{
		Used:      fmt.Sprintf("%.2f", totalUsed),
		Available: fmt.Sprintf("%.2f", available),
		Max:       fmt.Sprintf("%.2f", max),
		Timestamp: time.Now(),
	}, nil
}

func (d *DockerProvider) GetChildPools(ctx context.Context, poolID string) ([]*domain.ResourcePool, error) {
	// Docker provider does not support hierarchical pools in this example.
	// Adapt this method if you implement a custom labeling or grouping strategy for child pools.
	if poolID != "" {
		return []*domain.ResourcePool{}, nil
	}

	d.mutex.RLock()
	defer d.mutex.RUnlock()

	pools := make([]*domain.ResourcePool, 0, len(d.pools))
	for _, p := range d.pools {
		pools = append(pools, &domain.ResourcePool{
			ID:          p.ID,
			Name:        p.ID, // Using ID as Name in this example
			Technology:  d.GetTechnologyName(),
			Constraints: p.Constraints,
			Metadata: map[string]string{
				"label": p.Label,
			},
		})
	}

	return pools, nil
}

type DockerPool struct {
	ID          string
	Constraints domain.ResourceConstraints
	Containers  map[string]struct{} // Set of IDs of containers
	Label       string
}

func (d *DockerProvider) init() {
	d.technologyName = "docker"
	d.supportedResources = []domain.ResourceType{domain.CPU, domain.Memory, domain.Storage}
}

func NewDockerProvider() (*DockerProvider, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &DockerProvider{
		client: cli,
		pools:  make(map[string]*DockerPool),
	}, nil
}

func (d *DockerProvider) GetTechnologyName() string {
	return "docker"
}

func (d *DockerProvider) GetSupportedResources() []domain.ResourceType {
	return []domain.ResourceType{domain.CPU, domain.Memory, domain.Storage}
}

func (d *DockerProvider) CreatePool(ctx context.Context, pool *domain.ResourcePool) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Verificar que no exista el pool
	if _, exists := d.pools[pool.ID]; exists {
		return fmt.Errorf("pool %s already exists", pool.ID)
	}

	// Crear pool Docker con label única
	dockerPool := &DockerPool{
		ID:          pool.ID,
		Constraints: pool.Constraints,
		Containers:  make(map[string]struct{}),
		Label:       fmt.Sprintf("devops-platform.resource.pool=%s", pool.ID),
	}

	d.pools[pool.ID] = dockerPool
	return nil
}

func (d *DockerProvider) GetPool(ctx context.Context, poolID string) (*domain.ResourcePool, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	pool, exists := d.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("pool %s not found", poolID)
	}

	return &domain.ResourcePool{
		ID:          pool.ID,
		Technology:  d.GetTechnologyName(),
		Constraints: pool.Constraints,
	}, nil
}

func (d *DockerProvider) GetPoolUsage(ctx context.Context, poolID string) (map[domain.ResourceType]domain.ResourceUsage, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	pool, exists := d.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("pool %s not found", poolID)
	}

	// Obtener todos los contenedores del pool
	containers, err := d.client.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "label",
			Value: pool.Label,
		}),
	})
	if err != nil {
		return nil, err
	}

	// Acumular métricas
	usage := make(map[domain.ResourceType]domain.ResourceUsage)

	for _, container := range containers {
		stats, err := d.client.ContainerStats(ctx, container.ID, false)
		if err != nil {
			continue // O manejar error
		}
		defer stats.Body.Close()

		// Parsear estadísticas
		metrics, err := parseDockerStats(stats.Body)
		if err != nil {
			fmt.Printf("Error parsing stats for container %s: %v\n", container.ID, err)
			continue
		}

		// Acumular uso
		accumulateUsage(usage, metrics)
	}

	// Calcular disponibles basado en constraints
	result := make(map[domain.ResourceType]domain.ResourceUsage)
	for resType, constraint := range pool.Constraints {
		max := parseConstraint(constraint, resType)
		used := parseResource(usage[resType].Used, resType)

		result[resType] = domain.ResourceUsage{
			Used:      fmt.Sprintf("%.2f", used),
			Available: fmt.Sprintf("%.2f", max-used),
			Max:       constraint,
			Timestamp: time.Now(),
		}
	}

	return result, nil
}

// parseConstraint converts a resource constraint to a float value.
func parseConstraint(constraint string, resourceType domain.ResourceType) float64 {
	switch resourceType {
	case domain.CPU:
		// Handle CPU constraints (e.g., "2.5")
		value, err := strconv.ParseFloat(constraint, 64)
		if err != nil {
			return 0
		}
		return value
	case domain.Memory:
		// Handle memory constraints (e.g., "1Gi", "512Mi")
		return float64(parseMemoryConstraint(constraint))
	default:
		return 0
	}
}

// parseResource converts a resource usage to a float value.
func parseResource(resource string, resourceType domain.ResourceType) float64 {
	switch resourceType {
	case domain.CPU:
		// Handle CPU usage (e.g., "1.2")
		value, err := strconv.ParseFloat(resource, 64)
		if err != nil {
			return 0
		}
		return value
	case domain.Memory:
		// Handle memory usage (e.g., "536870912")
		value, err := strconv.ParseFloat(resource, 64)
		if err != nil {
			return 0
		}
		return value
	default:
		return 0
	}
}

func (d *DockerProvider) SetConstraints(ctx context.Context, poolID string, constraints domain.ResourceConstraints) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	pool, exists := d.pools[poolID]
	if !exists {
		return fmt.Errorf("pool %s not found", poolID)
	}

	// Actualizar constraints
	pool.Constraints = constraints

	// Actualizar contenedores existentes (implementación simplificada)
	containers, err := d.client.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "label",
			Value: pool.Label,
		}),
	})
	if err != nil {
		return err
	}

	for _, container := range containers {
		// Actualizar configuración del contenedor
		err := d.updateContainerResources(ctx, container.ID, constraints)
		if err != nil {
			return err
		}
	}

	return nil
}

// ************** Helper Functions **************

func (d *DockerProvider) updateContainerResources(ctx context.Context, containerID string, constraints domain.ResourceConstraints) error {
	// Convertir constraints a Docker resource limits
	updateConfig := container.UpdateConfig{
		Resources: container.Resources{
			CPUPeriod:  int64(parseCPUConstraint(constraints[domain.CPU]) * 100000),
			CPUQuota:   int64(parseCPUConstraint(constraints[domain.CPU]) * 100000),
			Memory:     parseMemoryConstraint(constraints[domain.Memory]),
			MemorySwap: -1, // Deshabilitar swap
		},
	}

	_, err := d.client.ContainerUpdate(ctx, containerID, updateConfig)
	return err
}

func parseCPUConstraint(constraint string) float64 {
	value, err := strconv.ParseFloat(constraint, 64)
	if err != nil {
		return 0
	}
	return value
}

// parseMemoryConstraint convierte una restricción de memoria en un valor entero
func parseMemoryConstraint(constraint string) int64 {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	value := strings.ToUpper(constraint)
	if strings.HasSuffix(value, "GB") {
		parsedValue, _ := strconv.ParseFloat(strings.TrimSuffix(value, "GB"), 64)
		return int64(parsedValue * GB)
	} else if strings.HasSuffix(value, "MB") {
		parsedValue, _ := strconv.ParseFloat(strings.TrimSuffix(value, "MB"), 64)
		return int64(parsedValue * MB)
	} else if strings.HasSuffix(value, "KB") {
		parsedValue, _ := strconv.ParseFloat(strings.TrimSuffix(value, "KB"), 64)
		return int64(parsedValue * KB)
	} else {
		parsedValue, _ := strconv.ParseFloat(value, 64)
		return int64(parsedValue)
	}
}

// parseDockerStats reads the JSON stats and extracts relevant metrics.
func parseDockerStats(r io.Reader) (map[domain.ResourceType]float64, error) {
	var stats dockerStats
	if err := json.NewDecoder(r).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode docker stats: %w", err)
	}

	metrics := make(map[domain.ResourceType]float64)

	// Calculate CPU usage percentage.
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	onlineCPUs := stats.CPUStats.OnlineCPUs
	if onlineCPUs == 0 {
		onlineCPUs = len(stats.CPUStats.CPUUsage.PercpuUsage)
	}

	if systemDelta > 0 && cpuDelta > 0 {
		metrics[domain.CPU] = (cpuDelta / systemDelta) * float64(onlineCPUs) * 100.0
	} else {
		metrics[domain.CPU] = 0.0 // Or some other appropriate default/error handling
	}

	// Memory usage (in bytes).
	metrics[domain.Memory] = float64(stats.MemoryStats.Usage)

	return metrics, nil
}

func accumulateUsage(usage map[domain.ResourceType]domain.ResourceUsage, metrics map[domain.ResourceType]float64) {
	for resourceType, value := range metrics {
		if _, ok := usage[resourceType]; !ok {
			usage[resourceType] = domain.ResourceUsage{
				Used:      "0",
				Available: "0",
				Max:       "0",
				Timestamp: time.Now(),
			}
		}
		// Convertir el valor a string y sumarlo al acumulado
		currentUsed, _ := strconv.ParseFloat(usage[resourceType].Used, 64)
		newUsed := currentUsed + value
		usage[resourceType] = domain.ResourceUsage{
			Used:      fmt.Sprintf("%.2f", newUsed), // Formatear a dos decimales
			Available: usage[resourceType].Available,
			Max:       usage[resourceType].Max,
			Timestamp: time.Now(),
		}
	}
}
