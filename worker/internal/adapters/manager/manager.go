package manager

import (
	"context"
	"dev.rubentxu.devops-platform/worker/internal/adapters/scheduler"
	"dev.rubentxu.devops-platform/worker/internal/adapters/store"
	"dev.rubentxu.devops-platform/worker/internal/adapters/worker"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"dev.rubentxu.devops-platform/worker/internal/ports"

	"fmt"
	"log"
	"sync"

	"time"

	"github.com/google/uuid"
)

// Manager manages task execution.
//
//go:generate mockery --name=Manager --output=mocks --case=underscore
type Manager struct {
	mu               sync.Mutex
	pendingTasksChan chan uuid.UUID
	taskDb           store.Store[domain.Task]
	taskOperationDb  store.Store[ports.TaskOperation]
	resourcePools    map[string]*ports.ResourcePool
	scheduler        ports.Scheduler
	worker           *worker.Worker
}

// New creates a new Manager instance.
func New(schedulerType string, dbType string, worker *worker.Worker, sizePendingsTask int) (*Manager, error) { // Recibe el Worker
	// ... (Creación del Scheduler y los Stores, como antes) ...
	// Crear Scheduler
	var currentSheduler ports.Scheduler
	switch schedulerType {
	case "greedy":
		currentSheduler = scheduler.NewGreedy()
	case "roundrobin":
		currentSheduler = scheduler.NewRoundRobin()
	default:
		currentSheduler = scheduler.NewEpvm() // Asegúrate de que NewEpvm exista
	}

	// Crear Stores
	var taskDb store.Store[domain.Task]
	var taskOperationDb store.Store[ports.TaskOperation]
	var err error // Declarar err aquí para que esté disponible en todo el bloque

	switch dbType {
	case "memory":
		taskDb = store.NewInMemoryStore[domain.Task]()

	case "persistent":
		taskDb, err = store.NewBoltDBStore[domain.Task]("tasks.db", 0600, "tasks")
		if err != nil {
			return nil, fmt.Errorf("unable to create task store: %w", err)
		}

	default:
		return nil, fmt.Errorf("invalid dbType: %currentSheduler", dbType)
	}
	taskOperationDb = store.NewInMemoryStore[ports.TaskOperation]()

	m := Manager{
		pendingTasksChan: make(chan uuid.UUID, sizePendingsTask), // Buffer para 1000 tareas
		taskDb:           taskDb,
		taskOperationDb:  taskOperationDb,
		resourcePools:    make(map[string]*ports.ResourcePool), // Se inicializa vacío
		scheduler:        currentSheduler,
		worker:           worker, // Guardar la instancia del Worker
	}

	return &m, nil
}

// AddResourcePool agrega un ResourcePool al Manager.
func (m *Manager) AddResourcePool(pool ports.ResourcePool) {
	m.resourcePools[pool.GetID()] = &pool
}

// AddTask añade una nueva Task a la cola de pendientes.
func (m *Manager) AddTask(taskDef domain.Task, ctx context.Context) (ports.TaskOperation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	log.Printf("Añadiendo tarea en manager %s", taskDef.ID)
	outputChan := make(chan domain.ProcessOutput, 100)
	stateChan := make(chan domain.State, 10) // Canal para el estado
	errChan := make(chan error, 1)           // Canal para errores

	execution := domain.TaskExecution{
		ID:         uuid.New(),
		TaskID:     taskDef.ID,
		Name:       taskDef.Name,
		State:      domain.Pending, // O Scheduled
		WorkerSpec: taskDef.WorkerSpec,
	}

	operation := ports.TaskOperation{
		Task:       execution,
		OutputChan: outputChan,
		StateChan:  stateChan, // Asignar el canal de estado
		ErrChan:    errChan,   // Asignar el canal de errores
		Ctx:        ctx,
	}

	err := m.taskOperationDb.Put(taskDef.ID.String(), operation)
	if err != nil {
		return ports.TaskOperation{}, fmt.Errorf("error al guardar la task operation: %w", err)
	}
	log.Printf("Tarea %s añadida al registro de tareas operables", taskDef.ID)

	err = m.taskDb.Put(taskDef.ID.String(), taskDef)
	if err != nil {
		return ports.TaskOperation{}, fmt.Errorf("error al guardar la tarea: %w", err)
	}

	select {
	case m.pendingTasksChan <- taskDef.ID: // Enviar ID al channel
		log.Printf("Tarea %s enviada al channel", taskDef.ID)
	default:
		return ports.TaskOperation{}, fmt.Errorf("cola llena")
	}

	return operation, nil
}

// SelectWorker elige un ResourcePool para una tarea.
func (m *Manager) SelectWorker(taskDefID uuid.UUID) (*ports.ResourcePool, error) {
	// Implementa la lógica para seleccionar un worker (ResourcePool)
	// Por ahora, una selección simple (el primero disponible).
	// TODO: Implementar una lógica de selección más sofisticada.
	if len(m.resourcePools) == 0 {
		return nil, fmt.Errorf("no hay ResourcePools disponibles")
	}

	// Usar el scheduler para seleccionar un ResourcePool.
	taskDef, err := m.taskDb.Get(taskDefID.String())
	if err != nil {
		return nil, fmt.Errorf("tarea no encontrada: %w", err)
	}
	log.Printf("Seleccionando un worker para la tarea %s", taskDef.ID)
	listPools := []*ports.ResourcePool{}
	for _, pool := range m.resourcePools {
		listPools = append(listPools, pool)
	}
	candidatePools := m.scheduler.SelectCandidateNodes(taskDef, listPools)
	log.Printf("Candidate pools: %v", candidatePools)

	scores := m.scheduler.Score(taskDef, candidatePools)
	log.Printf("Scores: %v", scores)

	selectedPool := m.scheduler.Pick(scores, candidatePools)

	if selectedPool == nil {
		return nil, fmt.Errorf("no se pudo seleccionar un ResourcePool")
	}
	log.Printf("Worker seleccionado: %s", (*selectedPool).GetID())
	return selectedPool, nil
}

// ProcessTasks procesa las tareas pendientes.
func (m *Manager) ProcessTasks() {
	for taskID := range m.pendingTasksChan { // Escuchar el channel
		log.Printf("Procesando: %s", taskID)
		go m.processTask(taskID)
	}
}

// processTask maneja la lógica de una sola tarea:  selección, lanzamiento y actualización.
// processTask ahora delega la ejecución al Worker.
func (m *Manager) processTask(taskDefID uuid.UUID) {
	log.Printf("Iniciando el procesamiento de la tarea %s", taskDefID)

	// 1. Obtener la definición de la tarea
	_, err := m.taskDb.Get(taskDefID.String())
	if err != nil {
		log.Printf("Error obteniendo la definición de la tarea %s: %v", taskDefID, err)
		return
	}

	// 2. Seleccionar un ResourcePool
	selectedPool, err := m.SelectWorker(taskDefID)
	if err != nil {
		log.Printf("Error seleccionando un worker para la tarea %s: %v", taskDefID, err)
		return
	}
	log.Printf("Worker seleccionado: %s", (*selectedPool).GetID())

	// 3. Obtener la configuración del ResourcePool
	if selectedPool == nil {
		log.Printf("Error: selectedPool is nil for task %s", taskDefID)
		return
	}

	// 4. Crear una TaskExecution
	operation, err := m.taskOperationDb.Get(taskDefID.String())
	if err != nil {
		log.Printf("Error obteniendo la tarea %s: %v", taskDefID, err)
		return
	}
	log.Printf("Tarea %s asignada al worker %s", taskDefID, (*selectedPool).GetID())

	resourceClient := (*selectedPool).GetResourceInstanceClient()
	operation.Client = resourceClient
	m.taskOperationDb.Put(taskDefID.String(), operation)
	// 5. Delegar la ejecución al Worker
	result := m.worker.AddTask(operation)
	log.Printf("Tarea %s enviada al worker", taskDefID)
	log.Printf("Result: %v", result)

	// 6. Actualizar el estado de la tarea
	if result != nil {
		log.Printf("Error al iniciar la tarea %s en el worker: %v", taskDefID, result.Error)
		operation.Task.State = domain.Failed
		operation.Task.Error = result.Error()
	} else {
		log.Printf("Tarea %s completada con éxito en el worker", taskDefID)
		operation.Task.State = domain.Completed
		operation.Task.FinishTime = time.Now()
	}

	log.Printf("Tarea %s procesada", taskDefID)
}


// GetTasks devuelve todas las Tasks (para la UI, por ejemplo).
func (m *Manager) GetTasks() ([]domain.Task, error) {
	return m.taskDb.List()

}

// --- Métodos relacionados con la salud de las tareas (Health Checks) ---
// (Estos métodos probablemente no cambian mucho, pero los incluyo para tener
//  el código completo).

// DoHealthChecks realiza las verificaciones de salud para todas las tareas.
func (m *Manager) DoHealthChecks() {
	m.doHealthChecks()
}

func (m *Manager) doHealthChecks() {
	//executions := m.GetTaskExecutions()
	//
	//for _, exec := range executions {
	//	err := m.checkTaskHealth(&exec)
	//	if err != nil {
	//		log.Printf("Error al verificar la salud de la tarea %s: %v", exec.TaskID, err)
	//	}
	//}
}

// checkTaskHealth realiza una verificación de salud para una TaskExecution.
func (m *Manager) checkTaskHealth(exec *domain.TaskExecution) error {
	// TODO: Implementar la lógica de verificación de salud real.
	//  Por ahora, solo imprimimos un mensaje.
	log.Printf("Verificando la salud de la tarea %s (Execution ID: %s)", exec.TaskID, exec.ID)
	return nil
}

func (m *Manager) StopTask(id string) error {
	log.Printf("Deteniendo la tarea %s", id)
	return m.worker.StopTask(id)
}
