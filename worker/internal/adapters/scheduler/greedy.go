package scheduler

import (
	"dev.rubentxu.devops-platform/worker/internal/adapters/resources"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"fmt"
	"math"
)

type Greedy struct {
	Name string
}

func NewGreedy() *Greedy {
	return &Greedy{Name: "greedy"}
}

func (g *Greedy) SelectCandidateNodes(t domain.Task, pools []*resources.ResourcePool) []*resources.ResourcePool {
	candidates := []*resources.ResourcePool{}
	for _, pool := range pools {
		if (*pool).Matches(t) {
			candidates = append(candidates, pool)
		}
	}
	return candidates
}

func (g *Greedy) Score(t domain.Task, pools []*resources.ResourcePool) map[string]float64 {
	scores := make(map[string]float64)
	for _, pool := range pools {
		stats, err := (*pool).GetStats()
		if err != nil {
			// Manejar el error (por ejemplo, asignar un score muy bajo)
			scores[fmt.Sprintf("%p", *pool)] = -1000.0
			continue
		}

		// Un ejemplo simple de puntuación:  más memoria libre = mejor score.
		// ¡Ajusta esto a tu lógica de "greedy"!  Podrías considerar CPU, disco, etc.
		//Normalizar los valores, para tener un valor de comparacion entre 0 y 1
		score := float64(stats.MemAvailableKb()) / float64(stats.MemTotalKb())
		scores[fmt.Sprintf("%p", *pool)] = score
	}
	return scores
}

func (g *Greedy) Pick(scores map[string]float64, candidates []*resources.ResourcePool) *resources.ResourcePool {
	if len(candidates) == 0 {
		return nil
	}

	var bestPool *resources.ResourcePool
	maxScore := -math.MaxFloat64 // Inicializar con el valor más bajo posible

	for _, pool := range candidates {
		poolAddr := fmt.Sprintf("%p", *pool)
		if score, ok := scores[poolAddr]; ok {
			if score > maxScore {
				maxScore = score
				bestPool = pool
			}
		}
	}

	return bestPool
}
