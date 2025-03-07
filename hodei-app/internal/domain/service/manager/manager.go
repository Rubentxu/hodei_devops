package manager

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"k8s.io/apimachinery/pkg/util/rand"

	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/worker"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/service/resource"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/service/scheduler"
	"github.com/pocketbase/pocketbase/core"

	"fmt"
	"log"
	"sync"

	"time"
)

// Manager manages task execution.
//
//go:generate mockery --name=Manager --output=mocks --case=underscore
type Manager struct {
	mu               sync.Mutex
	pendingTasksChan chan ports.TaskContext
	taskDb           ports.Store[model.Task]
	scheduler        ports.Scheduler
	worker           *worker.WorkerInstanceManager
	poolManager      *resource.ResourcePoolManager
}

// New creates a new Manager instance.
func New(schedulerType string, dbType string, worker *worker.WorkerInstanceManager, sizePendingsTask int, app core.App, poolManager *resource.ResourcePoolManager) (*Manager, error) {
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
	var taskDb ports.Store[model.Task]
	var err error // Declarar err aquí para que esté disponible en todo el bloque

	switch dbType {
	case "memory":
		taskDb = repository.NewCacheStore[model.Task]()

	case "persistent":
		taskDb, err = repository.NewPocketBaseStore[model.Task](app, "tasks")
		if err != nil {
			return nil, fmt.Errorf("unable to create task store: %w", err)
		}

	default:
		return nil, fmt.Errorf("invalid dbType: %currentSheduler", dbType)
	}

	m := Manager{
		pendingTasksChan: make(chan ports.TaskContext, sizePendingsTask), // Buffer para 1000 tareas
		taskDb:           taskDb,
		scheduler:        currentSheduler,
		worker:           worker, // Guardar la instancia del WorkerInstanceManager
		poolManager:      poolManager,
	}

	return &m, nil
}

// GetResourcePool obtiene un ResourcePool por su ID utilizando el ResourcePoolManager
func (m *Manager) GetResourcePool(id string) *ports.ResourcePool {
	if pool, exists := m.poolManager.GetActivePool(id); exists {
		return pool
	}
	return nil
}

// AddTask añade una nueva Execution a la cola de pendientes.
func (m *Manager) AddTask(taskDef model.Task, ctx context.Context) (ports.TaskContext, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	log.Printf("Añadiendo tarea en manager %s", taskDef.ID)
	outputChan := make(chan model.ProcessOutput, 100)
	stateChan := make(chan model.TaskState, 10) // Canal para el estado
	errChan := make(chan error, 1)

	// TODO: Recuperar WorkerDefinition de la base de datos a partir de taskDef.WorkerDefinitionID

	execution := model.TaskExecution{
		ID: model.NewAggregateID(),
		Metadata: model.NewMetadata(
			taskDef.Metadata.Name+randString(5),
			taskDef.Metadata.Description,
		),
		Status: model.ExecutionStatus{
			StartTime: time.Now(),
			State:     model.Pending,
		},
		// TODO: Guardar la definición de WorkerDefinition en la TaskExecution
	}

	taskContext := ports.TaskContext{
		Execution:  execution,
		OutputChan: outputChan,
		StateChan:  stateChan, // Asignar el canal de estado
		ErrChan:    errChan,   // Asignar el canal de errores
		Ctx:        ctx,
	}

	log.Printf("Tarea %s añadida al registro de tareas operables", taskDef.ID)

	err := m.taskDb.Put(taskDef.ID.String(), taskDef)
	if err != nil {
		return ports.TaskContext{}, fmt.Errorf("error al guardar la tarea: %w", err)
	}

	select {
	case m.pendingTasksChan <- taskContext: // Enviar ID al channel
		log.Printf("Tarea %s enviada al channel", taskDef.ID)
	default:
		return ports.TaskContext{}, fmt.Errorf("cola llena")
	}

	return taskContext, nil
}

func randString(n int) string {
	if n <= 0 {
		return ""
	}

	const lowercase = "abcdefghijklmnopqrstuvwxyz"
	const digits = "0123456789"
	const charset = lowercase + digits + "-"

	// Asegurar que el primer carácter sea una letra minúscula
	b := make([]byte, n)
	b[0] = lowercase[rand.Intn(len(lowercase))]

	// Resto de caracteres pueden ser letras minúsculas, números o guiones
	for i := 1; i < n; i++ {
		b[i] = charset[rand.Intn(len(charset))]
	}

	return string(b)
}

// SelectWorker elige un ResourcePool para una tarea.
func (m *Manager) SelectWorker(definition model.WorkerDefinition) (*ports.ResourcePool, error) {

	// Obtener la lista de pools activos del ResourcePoolManager
	activePools := m.poolManager.ListActivePools()
	if len(activePools) == 0 {
		return nil, fmt.Errorf("no hay ResourcePools disponibles")
	}

	log.Printf("Seleccionando un worker para el workerDefinition %s", definition.ID)

	candidatePools := m.scheduler.SelectCandidateNodes(definition, activePools)
	log.Printf("Candidate pools: %v", candidatePools)

	scores := m.scheduler.Score(candidatePools)
	log.Printf("Scores: %v", scores)

	selectedPool := m.scheduler.Pick(scores, candidatePools)
	if selectedPool == nil {
		return nil, fmt.Errorf("no se pudo seleccionar un ResourcePool")
	}

	log.Printf("WorkerInstanceManager seleccionado: %s", (*selectedPool).GetID())
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
// processTask ahora delega la ejecución al WorkerInstanceManager.
func (m *Manager) processTask(taskContext ports.TaskContext) {
	taskDefID := taskContext.Execution.ID
	log.Printf("Iniciando el procesamiento de la tarea %s", taskDefID)
	workerDefinition := taskContext.Execution.WorkerDef

	// 2. Seleccionar un ResourcePool usando el nuevo método que trabaja con el ResourcePoolManager
	selectedPool, err := m.SelectWorker(workerDefinition)
	if err != nil {
		log.Printf("Error seleccionando un worker para la tarea %s: %v", taskDefID, err)
		return
	}
	log.Printf("WorkerInstanceManager seleccionado: %s", (*selectedPool).GetID())

	// 3. Verificar que el pool seleccionado existe
	if selectedPool == nil {
		log.Printf("Error: selectedPool is nil for task %s", taskDefID)
		return
	}

	log.Printf("Tarea %s asignada al worker %s", taskDefID, (*selectedPool).GetID())

	resourceClient := (*selectedPool).GetResourceInstanceClient()
	taskContext.Client = resourceClient

	// 5. Delegar la ejecución al WorkerInstanceManager
	result := m.worker.AddTask(taskContext)
	log.Printf("Tarea %s enviada al worker", taskDefID)
	log.Printf("Result: %v", result)

	// 6. Actualizar el estado de la tarea
	if result != nil {
		log.Printf("Error al iniciar la tarea %s en el worker: %v", taskDefID, result.Error)
		taskContext.Execution.Status.State = model.Failed
		taskContext.Execution.Status.Message = result.Error()

	} else {
		log.Printf("Tarea %s completada con éxito en el worker", taskDefID)
		taskContext.Execution.Status.State = model.Completed
		taskContext.Execution.Status.EndTime = time.Now().UTC()
	}

	log.Printf("Tarea %s procesada", taskDefID)
}

// GetTasks devuelve todas las Tasks (para la UI, por ejemplo).
func (m *Manager) GetTasks() ([]model.Task, error) {
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
func (m *Manager) checkTaskHealth(exec *model.TaskExecution) error {
	// TODO: Implementar la lógica de verificación de salud real.
	//  Por ahora, solo imprimimos un mensaje.
	log.Printf("Verificando la salud de la tarea %s (Execution ID: %s)", exec.Task.ID, exec.ID)
	return nil
}

func (m *Manager) StopTask(taskContext ports.TaskContext) error {
	log.Printf("Deteniendo la tarea %s", taskContext.Execution.ID)
	return m.worker.StopTask(taskContext)
}
