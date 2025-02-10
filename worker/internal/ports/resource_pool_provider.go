package ports

import (
	"context"
	"dev.rubentxu.devops-platform/worker/internal/domain"
)

type ResourcePoolProvider interface {
	// Pool Management
	CreatePool(ctx context.Context, pool *domain.ResourcePool) error
	DeletePool(ctx context.Context, poolID string) error
	GetPool(ctx context.Context, poolID string) (*domain.ResourcePool, error)

	// Metrics
	GetPoolUsage(ctx context.Context, poolID string) (map[domain.ResourceType]domain.ResourceUsage, error)
	GetResourceUsage(ctx context.Context, poolID string, resourceType domain.ResourceType) (domain.ResourceUsage, error)

	// Constraints Enforcement
	SetConstraints(ctx context.Context, poolID string, constraints domain.ResourceConstraints) error

	// Technology Info
	GetSupportedResources() []domain.ResourceType
	GetTechnologyName() string

	// Hierarchy
	GetChildPools(ctx context.Context, poolID string) ([]*domain.ResourcePool, error)
}
