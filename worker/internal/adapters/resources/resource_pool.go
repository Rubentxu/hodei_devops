package resources

import (
	"context"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"dev.rubentxu.devops-platform/worker/internal/ports"
	"fmt"
	"sync"
)

type ResourceManager struct {
	providers  map[string]ports.ResourcePoolProvider
	poolsCache map[string]*domain.ResourcePool
	cacheMutex sync.RWMutex
}

func NewResourceManager() *ResourceManager {
	return &ResourceManager{
		providers:  make(map[string]ports.ResourcePoolProvider),
		poolsCache: make(map[string]*domain.ResourcePool),
	}
}

func (rm *ResourceManager) RegisterProvider(provider ports.ResourcePoolProvider) {
	rm.providers[provider.GetTechnologyName()] = provider
}

// ************** Common Operations **************

func (rm *ResourceManager) GetPoolUsage(ctx context.Context, poolID string) (map[domain.ResourceType]domain.ResourceUsage, error) {
	pool, err := rm.getPoolFromCache(poolID)
	if err != nil {
		return nil, err
	}

	provider, exists := rm.providers[pool.Technology]
	if !exists {
		return nil, fmt.Errorf("provider for technology %s not found", pool.Technology)
	}

	return provider.GetPoolUsage(ctx, poolID)
}

func (rm *ResourceManager) EnforceConstraints(ctx context.Context, poolID string, constraints domain.ResourceConstraints) error {
	pool, err := rm.getPoolFromCache(poolID)
	if err != nil {
		return err
	}

	provider, exists := rm.providers[pool.Technology]
	if !exists {
		return fmt.Errorf("provider for technology %s not found", pool.Technology)
	}

	return provider.SetConstraints(ctx, poolID, constraints)
}

// ************** Cache Management **************

func (rm *ResourceManager) RefreshPoolCache(ctx context.Context) error {
	rm.cacheMutex.Lock()
	defer rm.cacheMutex.Unlock()

	for tech, provider := range rm.providers {
		pools, err := provider.GetChildPools(ctx, "") // Obtiene todos los pools raíz
		if err != nil {
			return fmt.Errorf("error refreshing cache for %s: %v", tech, err)
		}

		for _, pool := range pools {
			rm.poolsCache[pool.ID] = pool
		}
	}
	return nil
}

func (rm *ResourceManager) getPoolFromCache(poolID string) (*domain.ResourcePool, error) {
	rm.cacheMutex.RLock()
	defer rm.cacheMutex.RUnlock()

	pool, exists := rm.poolsCache[poolID]
	if !exists {
		return nil, fmt.Errorf("pool %s not found in cache", poolID)
	}
	return pool, nil
}

// ************** Ejemplo de Implementación: Kubernetes **************

func main() {
	manager := NewResourceManager()
	manager.RegisterProvider(&KubernetesProvider{})

	// Actualizar caché de pools
	if err := manager.RefreshPoolCache(context.Background()); err != nil {
		panic(err)
	}

	// Obtener uso de un pool específico
	usage, err := manager.GetPoolUsage(context.Background(), "k8s-namespace-1")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Uso actual del pool:\n%+v\n", usage)

	// Establecer nuevas restricciones
	newConstraints := domain.ResourceConstraints{
		domain.CPU:    "1500m",
		domain.Memory: "16Gi",
	}
	if err := manager.EnforceConstraints(context.Background(), "k8s-namespace-1", newConstraints); err != nil {
		panic(err)
	}
}

func mainDocker() {
	// Inicializar provider Docker
	dockerProvider, err := NewDockerProvider()
	if err != nil {
		panic(err)
	}

	// Registrar en el ResourceManager
	manager := NewResourceManager()
	manager.RegisterProvider(dockerProvider)

	// Crear un nuevo pool Docker
	pool := &domain.ResourcePool{
		ID:         "high-perf-pool",
		Technology: "docker",
		Constraints: domain.ResourceConstraints{
			domain.CPU:    "4.0",
			domain.Memory: "16Gi",
		},
	}

	if err := dockerProvider.CreatePool(context.Background(), pool); err != nil {
		panic(err)
	}

	// Actualizar caché
	if err := manager.RefreshPoolCache(context.Background()); err != nil {
		panic(err)
	}

	// Obtener métricas del pool
	usage, err := manager.GetPoolUsage(context.Background(), "high-perf-pool")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Uso del pool Docker:\n")
	for resType, metric := range usage {
		fmt.Printf("%s: %s/%s (Available: %s)\n",
			resType,
			metric.Used,
			metric.Max,
			metric.Available)
	}

	// Actualizar constraints
	newConstraints := domain.ResourceConstraints{
		domain.CPU:    "8.0",
		domain.Memory: "32Gi",
	}

	if err := manager.EnforceConstraints(context.Background(), "high-perf-pool", newConstraints); err != nil {
		panic(err)
	}
}
