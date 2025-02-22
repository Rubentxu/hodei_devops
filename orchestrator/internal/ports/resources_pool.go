package ports

import (
	"dev.rubentxu.devops-platform/orchestrator/internal/domain"
)

type ResourcePool interface {
	GetID() string
	GetStats() (*domain.Stats, error)
	Matches(task domain.Task) bool
	GetResourceInstanceClient() ResourceIntanceClient
}
