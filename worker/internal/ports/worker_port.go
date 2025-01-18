package ports

import (
	"dev.rubentxu.devops-platform/worker/internal/domain"
)

// WorkerPort define el comportamiento que la aplicación espera
// de un "worker": arranque/parada de tareas, actualización, etc.
type WorkerPort interface {
	StartTask(t domain.Task) domain.TaskResult
	StopTask(t domain.Task) domain.TaskResult
	GetTasks() ([]*domain.Task, error)
	AddTask(t domain.Task)
}
