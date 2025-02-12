package ports

import "dev.rubentxu.devops-platform/worker/internal/domain"

type ResourcePool interface {
	GetStats() (*domain.Stats, error)
	Matches(task domain.Task) bool
	GetConfig() any //
}
