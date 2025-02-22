package worker

import (
	"context"
	"dev.rubentxu.devops-platform/orchestrator/internal/domain"
	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
	"fmt"
	"log"
	"sync"
	"time"
)

type StateTransition struct {
	FromState domain.HealthStatus
	ToState   domain.HealthStatus
	Action    func() error
}

type WorkerState struct {
	mu           sync.RWMutex
	currentState domain.HealthStatus
	transitions  map[domain.HealthStatus][]StateTransition
	stateChanged chan domain.HealthStatus
}

func NewWorkerState(workerInstance ports.WorkerInstance, taskID string) *WorkerState {
	ws := &WorkerState{
		currentState: domain.UNKNOWN,
		transitions:  make(map[domain.HealthStatus][]StateTransition),
		stateChanged: make(chan domain.HealthStatus, 1),
	}

	// Definir transiciones permitidas
	ws.AddTransition(domain.UNKNOWN, domain.RUNNING, nil)
	ws.AddTransition(domain.RUNNING, domain.HEALTHY, nil)
	ws.AddTransition(domain.RUNNING, domain.ERROR, nil)
	ws.AddTransition(domain.HEALTHY, domain.ERROR, nil)
	ws.AddTransition(domain.HEALTHY, domain.STOPPED, nil)

	// Función para detener el worker
	stopWorker := func() error {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if stopped, msg, err := workerInstance.Stop(stopCtx); err != nil {
			log.Printf("[%s] Error deteniendo worker: %v", taskID, err)
			return err
		} else if stopped {
			log.Printf("[%s] Worker detenido: %s", taskID, msg)
		}
		return nil
	}

	// Transiciones que requieren detener el worker
	ws.AddTransition(domain.RUNNING, domain.FINISHED, stopWorker)
	ws.AddTransition(domain.HEALTHY, domain.FINISHED, stopWorker)

	return ws
}

func (ws *WorkerState) AddTransition(from, to domain.HealthStatus, action func() error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.transitions[from] == nil {
		ws.transitions[from] = make([]StateTransition, 0)
	}
	ws.transitions[from] = append(ws.transitions[from], StateTransition{
		FromState: from,
		ToState:   to,
		Action:    action,
	})
}

func (ws *WorkerState) GetState() domain.HealthStatus {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.currentState
}

func (ws *WorkerState) SetState(newState domain.HealthStatus) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	currentState := ws.currentState
	if currentState == newState {
		return nil
	}

	// Verificar si la transición está permitida
	transitions := ws.transitions[currentState]
	for _, t := range transitions {
		if t.ToState == newState {
			if t.Action != nil {
				if err := t.Action(); err != nil {
					return fmt.Errorf("error en transición de estado %v -> %v: %w", currentState, newState, err)
				}
			}
			ws.currentState = newState
			select {
			case ws.stateChanged <- newState:
			default:
				// El canal está lleno, lo vaciamos y enviamos el nuevo estado
				select {
				case <-ws.stateChanged:
				default:
				}
				ws.stateChanged <- newState
			}
			return nil
		}
	}
	return fmt.Errorf("transición de estado no permitida: %v -> %v", currentState, newState)
}
