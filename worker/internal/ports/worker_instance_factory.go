package ports

import (
	"dev.rubentxu.devops-platform/worker/internal/domain"
)

// WorkerInstanceFactory define la interfaz para crear WorkerInstances
// según la configuración de la tarea.
type WorkerInstanceFactory interface {
	Create(config domain.WorkerInstanceSpec) (WorkerInstance, error)
}
