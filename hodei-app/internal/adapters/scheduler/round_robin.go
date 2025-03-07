package scheduler

import (
	"dev.rubentxu.devops-platform/orchestrator/internal/domain"
	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
	"fmt"
)

type RoundRobin struct {
	Name       string
	LastWorker int
}

func NewRoundRobin() *RoundRobin {
	return &RoundRobin{Name: "roundrobin", LastWorker: -1}
}

func (rr *RoundRobin) SelectCandidateNodes(t domain.Task, pools []*ports.ResourcePool) []*ports.ResourcePool {
	candidates := []*ports.ResourcePool{}
	for _, pool := range pools {
		if (*pool).Matches(t) {
			candidates = append(candidates, pool)
		}
	}
	return candidates
}

func (rr *RoundRobin) Score(t domain.Task, pools []*ports.ResourcePool) map[string]float64 {
	scores := make(map[string]float64)
	for _, pool := range pools { //No se necesita score en roundrobin
		scores[fmt.Sprintf("%p", *pool)] = 0.0 // Usar la dirección como identificador único.
	}
	return scores
}

func (rr *RoundRobin) Pick(scores map[string]float64, candidates []*ports.ResourcePool) *ports.ResourcePool {
	if len(candidates) == 0 {
		return nil
	}

	rr.LastWorker = (rr.LastWorker + 1) % len(candidates)
	return candidates[rr.LastWorker]
}
