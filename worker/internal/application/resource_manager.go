package application

import (
	"context"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"dev.rubentxu.devops-platform/worker/internal/ports"
	"fmt"
	"sync"
)

// ResourceManager manages resource pools and interacts with resource providers.
type ResourceManager struct {
	providers  map[string]ports.ResourcePoolProvider
	poolsCache map[string]*domain.ResourcePool
	cacheMutex sync.RWMutex
}

// NewResourceManager creates a new ResourceManager.
func NewResourceManager() *ResourceManager {
	return &ResourceManager{
		providers:  make(map[string]ports.ResourcePoolProvider),
		poolsCache: make(map[string]*domain.ResourcePool),
	}
}

// RegisterProvider registers a new resource pool provider.
func (rm *ResourceManager) RegisterProvider(provider ports.ResourcePoolProvider) {
	rm.providers[provider.GetTechnologyName()] = provider
}

// ************** Common Operations **************

// GetPoolUsage retrieves the usage metrics for a given pool.
func (rm *ResourceManager) GetPoolUsage(ctx context.Context, poolID string) (map[domain.ResourceType]domain.ResourceUsage, error) {
	pool, err := rm.getPoolFromCacheOrProvider(ctx, poolID)
	if err != nil {
		return nil, err
	}

	provider, exists := rm.providers[pool.Technology]
	if !exists {
		return nil, fmt.Errorf("provider for technology %s not found", pool.Technology)
	}

	return provider.GetPoolUsage(ctx, poolID)
}

// EnforceConstraints sets resource constraints on a given pool.
func (rm *ResourceManager) EnforceConstraints(ctx context.Context, poolID string, constraints domain.ResourceConstraints) error {
	pool, err := rm.getPoolFromCacheOrProvider(ctx, poolID)
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

// RefreshPoolCache updates the in-memory cache of resource pools.
// It fetches pools from all registered providers and handles hierarchical relationships.
func (rm *ResourceManager) RefreshPoolCache(ctx context.Context) error {
	rm.cacheMutex.Lock()
	defer rm.cacheMutex.Unlock()

	newPoolsCache := make(map[string]*domain.ResourcePool)
	for tech, provider := range rm.providers {
		pools, err := provider.GetChildPools(ctx, "") // Get all root pools
		if err != nil {
			return fmt.Errorf("error refreshing cache for %s: %v", tech, err)
		}

		for _, pool := range pools {
			newPoolsCache[pool.ID] = pool
			rm.populateChildPoolsRecursive(ctx, provider, pool, newPoolsCache) // Handle children recursively
		}
	}

	rm.poolsCache = newPoolsCache
	return nil
}

// populateChildPoolsRecursive recursively fetches and adds child pools to the cache.
func (rm *ResourceManager) populateChildPoolsRecursive(ctx context.Context, provider ports.ResourcePoolProvider, parent *domain.ResourcePool, cache map[string]*domain.ResourcePool) {
	childPools, err := provider.GetChildPools(ctx, parent.ID)
	if err != nil {
		// Log the error or handle it appropriately
		fmt.Printf("Error getting child pools for %s: %v\n", parent.ID, err)
		return
	}

	for _, child := range childPools {
		child.ParentPoolID = parent.ID // Set the parent ID for hierarchical reference
		cache[child.ID] = child
		rm.populateChildPoolsRecursive(ctx, provider, child, cache) // Recursively populate grandchild pools
	}
}

// getPoolFromCache retrieves a resource pool from the cache.
func (rm *ResourceManager) getPoolFromCache(poolID string) (*domain.ResourcePool, error) {
	rm.cacheMutex.RLock()
	defer rm.cacheMutex.RUnlock()

	pool, exists := rm.poolsCache[poolID]
	if !exists {
		return nil, fmt.Errorf("pool %s not found in cache", poolID)
	}
	return pool, nil
}

func (rm *ResourceManager) GetAllPoolsFromCache() ([]*domain.ResourcePool, error) {
	rm.cacheMutex.RLock()
	defer rm.cacheMutex.RUnlock()

	var pools []*domain.ResourcePool
	for _, pool := range rm.poolsCache {
		pools = append(pools, pool)
	}
	return pools, nil
}

// getPoolFromCacheOrProvider retrieves a pool from the cache, or fetches it from the provider if not found in the cache.
func (rm *ResourceManager) getPoolFromCacheOrProvider(ctx context.Context, poolID string) (*domain.ResourcePool, error) {
	rm.cacheMutex.RLock()
	pool, exists := rm.poolsCache[poolID]
	rm.cacheMutex.RUnlock()

	if exists {
		return pool, nil
	}

	// If not found in cache, try to fetch from providers (this is less efficient)
	for _, provider := range rm.providers {
		pool, err := provider.GetPool(ctx, poolID)
		if err == nil {
			rm.cacheMutex.Lock()
			rm.poolsCache[pool.ID] = pool
			rm.cacheMutex.Unlock()
			return pool, nil
		}
	}

	return nil, fmt.Errorf("pool %s not found", poolID)
}

// CreatePool creates a new resource pool using the appropriate provider.
func (rm *ResourceManager) CreatePool(ctx context.Context, pool *domain.ResourcePool) error {
	provider, exists := rm.providers[pool.Technology]
	if !exists {
		return fmt.Errorf("provider for technology %s not found", pool.Technology)
	}

	err := provider.CreatePool(ctx, pool)
	if err != nil {
		return err
	}

	// Update cache after successful creation
	rm.cacheMutex.Lock()
	rm.poolsCache[pool.ID] = pool
	rm.cacheMutex.Unlock()

	return nil
}

// DeletePool deletes a resource pool using the appropriate provider.
func (rm *ResourceManager) DeletePool(ctx context.Context, poolID string) error {
	pool, err := rm.getPoolFromCacheOrProvider(ctx, poolID)
	if err != nil {
		return err
	}

	provider, exists := rm.providers[pool.Technology]
	if !exists {
		return fmt.Errorf("provider for technology %s not found", pool.Technology)
	}

	err = provider.DeletePool(ctx, poolID)
	if err != nil {
		return err
	}

	// Update cache after successful deletion
	rm.cacheMutex.Lock()
	delete(rm.poolsCache, poolID)
	rm.cacheMutex.Unlock()

	return nil
}

// GetResourceUsage retrieves the usage of a specific resource type within a pool.
func (rm *ResourceManager) GetResourceUsage(ctx context.Context, poolID string, resourceType domain.ResourceType) (domain.ResourceUsage, error) {
	pool, err := rm.getPoolFromCacheOrProvider(ctx, poolID)
	if err != nil {
		return domain.ResourceUsage{}, err
	}

	provider, exists := rm.providers[pool.Technology]
	if !exists {
		return domain.ResourceUsage{}, fmt.Errorf("provider for technology %s not found", pool.Technology)
	}

	return provider.GetResourceUsage(ctx, poolID, resourceType)
}

// GetSupportedResources retrieves the resource types supported by a specific technology.
func (rm *ResourceManager) GetSupportedResources(technology string) ([]domain.ResourceType, error) {
	provider, exists := rm.providers[technology]
	if !exists {
		return nil, fmt.Errorf("provider for technology %s not found", technology)
	}

	return provider.GetSupportedResources(), nil
}

// UpdatePoolMetadata updates the metadata of a specific pool.
func (rm *ResourceManager) UpdatePoolMetadata(ctx context.Context, poolID string, key, value string) error {
	pool, err := rm.getPoolFromCacheOrProvider(ctx, poolID)
	if err != nil {
		return err
	}

	pool.UpdateMetadata(key, value)
	return nil
}

// DeletePoolMetadata removes a specific metadata entry from a pool.
func (rm *ResourceManager) DeletePoolMetadata(ctx context.Context, poolID string, key string) error {
	pool, err := rm.getPoolFromCacheOrProvider(ctx, poolID)
	if err != nil {
		return err
	}

	pool.DeleteMetadata(key)
	return nil
}
