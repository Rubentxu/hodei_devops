package ports

import (
	"dev.rubentxu.devops-platform/orchestrator/internal/domain"
)

const (
	// LIEB square ice constant
	// https://en.wikipedia.org/wiki/Lieb%27s_square_ice_constant
	LIEB = 1.53960071783900203869
)

type Scheduler interface {
	SelectCandidateNodes(t domain.Task, pools []*ResourcePool) []*ResourcePool
	Score(t domain.Task, pools []*ResourcePool) map[string]float64
	Pick(scores map[string]float64, candidates []*ResourcePool) *ResourcePool
}
