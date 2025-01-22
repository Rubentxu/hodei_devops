package ports

import (
	"dev.rubentxu.devops-platform/worker/internal/domain"
)

// WorkerFactory define la interfaz para crear WorkerInstances
// según la configuración de la tarea.
type WorkerFactory interface {
	Create(task domain.Task) (WorkerInstance, error)
}
