package domain

import (
	"sync"
	"time"
)

type ResourceType string

const (
	CPU       ResourceType = "cpu"
	Memory    ResourceType = "memory"
	Storage   ResourceType = "storage"
	Network   ResourceType = "network"
	GPU       ResourceType = "gpu"
	Instances ResourceType = "instances"
)

type ResourceConstraints map[ResourceType]string // Valores en formato tecnológico específico

type ResourceUsage struct {
	Used      string
	Available string
	Max       string
	Timestamp time.Time
	Error     string // Para manejar estados de error
}

type ResourcePool struct {
	ID           string
	Name         string
	Technology   string
	Constraints  ResourceConstraints
	ParentPoolID string            // Para jerarquías
	Metadata     map[string]string // Etiquetas, annotations
	mutex        sync.RWMutex      // Para concurrencia

	// Campo para cachear última métrica
	lastUsage map[ResourceType]ResourceUsage
}

// GetLastUsage retrieves the last cached usage for a specific resource type.
// It acquires a read lock to ensure thread safety.
func (rp *ResourcePool) GetLastUsage(resourceType ResourceType) (ResourceUsage, bool) {
	rp.mutex.RLock()
	defer rp.mutex.RUnlock()

	usage, ok := rp.lastUsage[resourceType]
	return usage, ok
}

// SetLastUsage updates the last cached usage for a specific resource type.
// It acquires a write lock to ensure thread safety.
func (rp *ResourcePool) SetLastUsage(resourceType ResourceType, usage ResourceUsage) {
	rp.mutex.Lock()
	defer rp.mutex.Unlock()

	if rp.lastUsage == nil {
		rp.lastUsage = make(map[ResourceType]ResourceUsage)
	}
	rp.lastUsage[resourceType] = usage
}

// UpdateMetadata updates the metadata of the resource pool.
// It acquires a write lock for thread safety.
func (rp *ResourcePool) UpdateMetadata(key, value string) {
	rp.mutex.Lock()
	defer rp.mutex.Unlock()

	if rp.Metadata == nil {
		rp.Metadata = make(map[string]string)
	}
	rp.Metadata[key] = value
}

// DeleteMetadata removes a metadata entry from the resource pool.
// It acquires a write lock for thread safety.
func (rp *ResourcePool) DeleteMetadata(key string) {
	rp.mutex.Lock()
	defer rp.mutex.Unlock()

	delete(rp.Metadata, key)
}
