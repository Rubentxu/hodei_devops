package scheduler

import (
	"dev.rubentxu.devops-platform/orchestrator/internal/domain"
	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
	"fmt"
	"log"
	"math"
)

type Greedy struct {
	Name string
}

func NewGreedy() *Greedy {
	return &Greedy{Name: "greedy"}
}

func (g *Greedy) SelectCandidateNodes(t domain.Task, pools []*ports.ResourcePool) []*ports.ResourcePool {
	candidates := []*ports.ResourcePool{}
	for _, pool := range pools {
		if (*pool).Matches(t) {
			candidates = append(candidates, pool)
		}
	}
	return candidates
}

func (g *Greedy) Score(t domain.Task, pools []*ports.ResourcePool) map[string]float64 {
	log.Println("Running Greedy Score")
	scores := make(map[string]float64)
	for _, pool := range pools {
		stats, err := (*pool).GetStats()
		if err != nil {
			// Manejar el error (por ejemplo, asignar un score muy bajo)
			scores[fmt.Sprintf("%p", *pool)] = -1000.0
			continue
		}
		log.Printf("ResourcePool Stats : %s - %f %f\n", (*pool).GetID(), stats.MemAvailableKb(), stats.MemTotalKb())

		// Un ejemplo simple de puntuación:  más memoria libre = mejor score.
		// ¡Ajusta esto a tu lógica de "greedy"!  Podrías considerar CPU, disco, etc.
		//Normalizar los valores, para tener un valor de comparacion entre 0 y 1
		score := float64(stats.MemAvailableKb()) / float64(stats.MemTotalKb())
		if stats.MemTotalKb() == 0 {
			score = 0.0 // Valor por defecto en caso de división por cero
		} else {
			score = float64(stats.MemAvailableKb()) / float64(stats.MemTotalKb())
		}
		log.Printf("Greedy Score: %s - %f\n", (*pool).GetID(), score)
		scores[fmt.Sprintf("%p", *pool)] = score
	}
	return scores
}

func (g *Greedy) Pick(scores map[string]float64, candidates []*ports.ResourcePool) *ports.ResourcePool {
	if len(candidates) == 0 {
		return nil
	}

	var bestPool *ports.ResourcePool
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
