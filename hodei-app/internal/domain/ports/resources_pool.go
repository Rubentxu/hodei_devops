package ports

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
)

type ResourcePool interface {
	GetID() string
	GetStats() (*model.Stats, error)
	Matches(definition model.WorkerDefinition) bool
	GetResourceInstanceClient() ResourceIntanceClient
}

// TemplateStoreAccessor proporciona acceso directo al store de templates
// Esto permite operaciones avanzadas como listar o eliminar templates sin tener que
// añadir estos métodos a la interfaz ResourcePool
type TemplateStoreAccessor interface {
	GetTemplateStore() Store[WorkerTemplate]
}

type WorkerTemplate struct {
	ID         string
	Template   string
	WorkerSpec model.WorkerSpec
}

// ResourcePoolFactory define una interfaz para crear instancias de ResourcePool
// a partir de una configuración
type ResourcePoolFactory interface {
	// CreateResourcePool crea una instancia de ResourcePool a partir de una configuración
	CreateResourcePool(resourcesDef *model.ResourcePoolDef) (ResourcePool, error)
	CreateDefaultResourcePool() (ResourcePool, error)
}

type ResourcePoolService interface {
	// Métodos existentes de gestión de definiciones
	CreateResourcePool(ctx context.Context, resourceDef *model.ResourcePoolDef) (*model.ResourcePoolDef, error)
	UpdateResourcePool(ctx context.Context, id model.AggregateID, updates *model.ResourcePoolDef) error
	DeleteResourcePool(ctx context.Context, id model.AggregateID) error
	GetResourcePool(ctx context.Context, id model.AggregateID) (*model.ResourcePoolDef, error)
	ListResourcePools(ctx context.Context, criteria SearchCriteria) (SearchResult[*model.ResourcePoolDef], error)

	// Métodos de gestión de instancias
	CreateResourcePoolInstance(ctx context.Context, id model.AggregateID) (*ResourcePool, error)
	CreateAllResourcePools(ctx context.Context) error

	// Nuevos métodos para gestión de pools activos
	GetActivePool(id string) (*ResourcePool, bool)
	ListActivePools() []*ResourcePool
}
