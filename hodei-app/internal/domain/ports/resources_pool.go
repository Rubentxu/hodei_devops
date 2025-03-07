package ports

import (
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
	Matches(task model.Task) bool
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
