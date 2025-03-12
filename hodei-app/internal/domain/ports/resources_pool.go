package ports

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
)

// ResourcePoolConfig es la interfaz común para todas las configuraciones de ResourcePool
type ResourcePoolConfig interface {
	GetType() string        // Devuelve el tipo de ResourcePool (docker, kubernetes, etc)
	GetName() string        // Devuelve un nombre único para esta configuración
	GetDescription() string // Devuelve una descripción de esta configuración
}

type ResourcePool interface {
	GetID() string
	GetStats() (*model.Stats, error)
	Matches(definition model.WorkerDefinition) bool
	GetResourceInstanceClient() ResourceIntanceClient
	GetWorkerTemplate(id string) (WorkerTemplate, error)
	AddWorkerTemplate(template WorkerTemplate) error
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
	CreateResourcePool(config map[string]interface{}, templateStore Store[WorkerTemplate]) (ResourcePool, error)
	CreateDefaultResourcePool() (ResourcePoolConfig, error)
}

type ResourcePoolService interface {
	CreateResourcePool(ctx context.Context, resourceDef *model.ResourcePoolDef) (*model.ResourcePoolDef, error)
	UpdateResourcePool(ctx context.Context, id model.AggregateID, updates *model.ResourcePoolDef) error
	DeleteResourcePool(ctx context.Context, id model.AggregateID) error
	GetResourcePool(ctx context.Context, id model.AggregateID) (*model.ResourcePoolDef, error)
	ListResourcePools(ctx context.Context, criteria SearchCriteria) (SearchResult[*model.ResourcePoolDef], error)
	CreateResourcePoolInstance(ctx context.Context, id model.AggregateID) (ResourcePool, error)
}
