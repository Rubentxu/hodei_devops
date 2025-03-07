package worker

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"fmt"
	"log"
	"sync"
	"time"
)

type StateTransition struct {
	FromState model.HealthStatus
	ToState   model.HealthStatus
	Action    func() error
}

type WorkerState struct {
	mu           sync.RWMutex
	currentState model.HealthStatus
	transitions  map[model.HealthStatus][]StateTransition
	stateChanged chan model.HealthStatus
}

func NewWorkerState(workerInstance ports.WorkerInstance, taskID string) *WorkerState {
	ws := &WorkerState{
		currentState: model.UNKNOWN,
		transitions:  make(map[model.HealthStatus][]StateTransition),
		stateChanged: make(chan model.HealthStatus, 1),
	}

	// Definir transiciones permitidas
	ws.AddTransition(model.UNKNOWN, model.RUNNING, nil)
	ws.AddTransition(model.RUNNING, model.HEALTHY, nil)
	ws.AddTransition(model.RUNNING, model.ERROR, nil)
	ws.AddTransition(model.HEALTHY, model.ERROR, nil)
	ws.AddTransition(model.HEALTHY, model.STOPPED, nil)

	// Función para detener el worker
	stopWorker := func() error {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if stopped, msg, err := workerInstance.Stop(stopCtx); err != nil {
			log.Printf("[%s] Error deteniendo worker: %v", taskID, err)
			return err
		} else if stopped {
			log.Printf("[%s] WorkerInstanceManager detenido: %s", taskID, msg)
		}
		return nil
	}

	// Transiciones que requieren detener el worker
	ws.AddTransition(model.RUNNING, model.FINISHED, stopWorker)
	ws.AddTransition(model.HEALTHY, model.FINISHED, stopWorker)

	return ws
}

func (ws *WorkerState) AddTransition(from, to model.HealthStatus, action func() error) {
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

func (ws *WorkerState) GetState() model.HealthStatus {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.currentState
}

func (ws *WorkerState) SetState(newState model.HealthStatus) error {
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
