package scheduler

import (
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"dev.rubentxu.devops-platform/worker/internal/ports"
	"fmt"
)

// Enhanced PVM (Parallel Virtual Machine) Algorithm
//
// Implementation of the E-PVM algorithm laid out in http://www.cnds.jhu.edu/pub/papers/mosix.pdf.
// The algorithm calculates the "marginal cost" of assigning a task to a machine. In the paper and
// in this implementation, the only resources considered for calculating a task's marginal cost are
// memory and cpu.

type Epvm struct {
	Name string
	// Puedes agregar campos aquí para mantener el estado de EPVM, como
	// los modelos de predicción, etc.
}

func NewEpvm() *Epvm {
	return &Epvm{Name: "epvm"}
}
func (e *Epvm) SelectCandidateNodes(t domain.Task, pools []*ports.ResourcePool) []*ports.ResourcePool {
	candidates := []*ports.ResourcePool{}
	for _, pool := range pools {
		if (*pool).Matches(t) {
			candidates = append(candidates, pool)
		}
	}
	return candidates
}

func (e *Epvm) Score(t domain.Task, pools []*ports.ResourcePool) map[string]float64 {
	scores := make(map[string]float64)
	// Aquí va la lógica de puntuación de EPVM.  Esto es lo más complejo
	// y específico de tu implementación de EPVM.  Necesitarás:
	// 1. Obtener métricas históricas (probablemente de una base de datos de series temporales).
	// 2. Entrenar un modelo (por ejemplo, una red neuronal, un modelo ARIMA, etc.).
	// 3. Usar el modelo para predecir el rendimiento futuro.
	// 4. Asignar un score basado en la predicción.

	for _, pool := range pools {
		// Ejemplo MUY básico (solo para ilustrar la estructura, no funciona)
		_, err := (*pool).GetStats() //Necesitarias obtener datos historicos en vez de los actuales
		if err != nil {
			scores[fmt.Sprintf("%p", *pool)] = -1000.0 // Manejar errores
			continue
		}
		// score := ...  // Aquí iría tu lógica de predicción y puntuación.
		scores[fmt.Sprintf("%p", *pool)] = 0.5 // Puntuación de ejemplo
	}

	return scores
}

func (e *Epvm) Pick(scores map[string]float64, candidates []*ports.ResourcePool) *ports.ResourcePool {
	// La lógica de Pick para EPVM podría ser similar a la de Greedy (elegir el mejor score),
	// pero podrías considerar otros factores, como la confianza de la predicción.
	if len(candidates) == 0 {
		return nil
	}

	var bestPool *ports.ResourcePool
	maxScore := -10000000000000.0 // Inicializar con el valor más bajo posible

	for _, pool := range candidates {
		poolAddr := fmt.Sprintf("%p", *pool) // Usar la dirección como un string unico

		if score, ok := scores[poolAddr]; ok {
			if score > maxScore {
				maxScore = score
				bestPool = pool
			}
		}
	}

	return bestPool
}
