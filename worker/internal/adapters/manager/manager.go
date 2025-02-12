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
	"net/http"

	"time"

	"github.com/docker/go-connections/nat"
	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
)

type Manager struct {
	pendingTasks    queue.Queue // Cola de TaskDefinition IDs (uuid.UUID)
	taskDb          store.Store[domain.Task]
	taskOperationDb store.Store[ports.TaskOperation]
	executionDb     store.Store[domain.TaskExecution]
	resourcePools   []*ports.ResourcePool
	scheduler       ports.Scheduler
	worker          *worker.Worker
}

func New(schedulerType string, dbType string, worker *worker.Worker) (*Manager, error) { // Recibe el Worker
	// ... (Creación del Scheduler y los Stores, como antes) ...
	// Crear Scheduler
	var s ports.Scheduler
	switch schedulerType {
	case "greedy":
		s = scheduler.NewGreedy()
	case "roundrobin":
		s = scheduler.NewRoundRobin()
	default:
		s = scheduler.NewEpvm() // Asegúrate de que NewEpvm exista
	}

	// Crear Stores
	var taskDb store.Store[ports.TaskOperation]
	var executionDb store.Store[domain.TaskExecution]
	var err error // Declarar err aquí para que esté disponible en todo el bloque

	switch dbType {
	case "memory":
		taskEDb = store.NewInMemoryStore[domain.Task]()
		executionDb = store.NewInMemoryStore[domain.TaskExecution]()
	case "persistent":
		taskDb, err = store.NewPersistentStore[domain.Task]("tasks.db", 0600, "tasks")
		if err != nil {
			return nil, fmt.Errorf("unable to create task store: %w", err)
		}
		executionDb, err = store.NewPersistentStore[domain.TaskExecution]("executions.db", 0600, "executions")
		if err != nil {
			return nil, fmt.Errorf("unable to create execution store: %w", err)
		}
	default:
		return nil, fmt.Errorf("invalid dbType: %s", dbType)
	}

	m := Manager{
		pendingTasks:  *queue.New(),
		taskDb:        taskDb,
		executionDb:   executionDb,
		resourcePools: []*ports.ResourcePool{}, // Se inicializa vacío
		scheduler:     s,
		worker:        worker, // Guardar la instancia del Worker
	}

	return &m, nil
}

// AddResourcePool agrega un ResourcePool al Manager.
func (m *Manager) AddResourcePool(pool ports.ResourcePool) {
	m.resourcePools = append(m.resourcePools, &pool)
}

// AddTask añade una nueva Task a la cola de pendientes.
func (m *Manager) AddTask(taskDef domain.Task) (<-chan *domain.ProcessOutput, error) {
	outputChan := make(chan *domain.ProcessOutput, 100)
	errChan := make(chan error, 1)
	ctx := context.Background()

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
		Ctx:        ctx,
		ErrChan:    errChan,
	}

	m.taskDb.Put(taskDef.ID.String(), taskDef)            // Guardar la Task
	m.taskOperationDb.Put(taskDef.ID.String(), operation) // Guardar la Task
	m.pendingTasks.Enqueue(taskDef.ID)                    // Encolar el ID
	log.Printf("Added task definition %s to pending queue", taskDef.ID)
}

// SelectWorker elige un ResourcePool para una tarea.
func (m *Manager) SelectWorker(taskDefID uuid.UUID) (*ports.ResourcePool, error) {
	// 1. Obtener la Task
	taskDef, err := m.taskDb.Get(taskDefID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get task definition: %w", err)
	}

	// 2. Seleccionar candidatos
	candidates := m.scheduler.SelectCandidateNodes(taskDef, m.resourcePools) //Pasar el ID
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available candidates match resource request for task %v", taskDefID)
	}

	// 3. Calcular scores
	scores := m.scheduler.Score(taskDef, candidates) //Pasar ID
	if len(scores) == 0 {
		return nil, fmt.Errorf("no scores returned for task %v", taskDefID)
	}

	// 4. Elegir el mejor
	selectedPool := m.scheduler.Pick(scores, candidates)
	return selectedPool, nil
}

// ProcessTasks procesa las tareas pendientes.
func (m *Manager) ProcessTasks() {
	for {
		if m.pendingTasks.Len() > 0 {
			taskDefID := m.pendingTasks.Dequeue().(uuid.UUID)
			m.processTask(taskDefID)
		} else {
			log.Println("No work in the queue")
		}
		time.Sleep(10 * time.Second)
	}
}

// processTask maneja la lógica de una sola tarea:  selección, lanzamiento y actualización.
// processTask ahora delega la ejecución al Worker.
func (m *Manager) processTask(taskDefID uuid.UUID) {
	log.Printf("Processing task definition %s", taskDefID)

	// 1. Obtener la TaskDefinition
	taskDef, err := m.taskDb.Get(taskDefID.String())
	if err != nil {
		log.Printf("Error getting task definition: %v", err)
		return
	}

	// 2. Seleccionar un ResourcePool (todavía lo necesitamos para saber *dónde* ejecutar)
	selectedPool, err := m.SelectWorker(taskDefID)
	if err != nil {
		log.Printf("Error selecting worker: %v", err)
		return
	}

	// 3. Obtener la configuración del ResourcePool
	poolConfig := (*selectedPool).GetConfig()

	// 4. Crear una TaskExecution

	m.executionDb.Put(execution.ID.String(), execution) // Guardar la ejecución

	// 6. Delegar la ejecución al Worker  (¡CORREGIDO!)
	outputChan, err := m.worker.AddTask(context.Background(), execution)
	if err != nil {
		log.Printf("Error adding task to worker: %v", err)
		execution.State = domain.Failed // Actualizar el estado de la ejecución
		m.executionDb.Put(execution.ID.String(), execution)
		return
	}

}

// UpdateTasks actualiza el estado de las tareas (ejecuciones).
// ESTE MÉTODO AHORA ES MÁS SENCILLO
func (m *Manager) UpdateTasks() {
	for {
		log.Println("Checking for task updates")
		m.updateTasks() //Ya no se necesitan parametros
		log.Println("Task updates completed")
		time.Sleep(15 * time.Second)
	}
}

func (m *Manager) updateTasks() {
	// Obtener todas las ejecuciones
	executions, err := m.executionDb.List()
	if err != nil {
		log.Printf("Error getting task executions: %v", err)
		return
	}

	// Iterar sobre las ejecuciones y actualizarlas (si es necesario)
	for _, exec := range executions {
		// Aqui ya tienes las ejecuciones, no necesitas hacer nada mas, a menos que quieras
		// verificar si el worker sigue vivo, etc.  Pero el estado principal
		// ya se actualiza en monitorTask.

		// Ejemplo de actualización (si fuera necesario):
		// if exec.State == domain.Running {
		//  	// Podrías hacer un ping al worker para verificar si la tarea sigue viva
		//  	// (pero esto debería estar en doHealthChecks, no aquí).
		// }
		updatedExec, _ := m.executionDb.Get(exec.ID.String())

		if updatedExec.State != exec.State { //Si el estado cambio
			m.executionDb.Put(exec.ID.String(), exec)
		}
	}
}

// GetTasks devuelve todas las TaskExecutions.
func (m *Manager) GetTaskExecutions() []domain.TaskExecution {
	executions, err := m.executionDb.List()
	if err != nil {
		log.Printf("Error getting task executions: %v", err)
		return nil
	}
	return executions
}

// GetTasks devuelve todas las Tasks (para la UI, por ejemplo).
func (m *Manager) GetTasks() []domain.Task {
	tasks, err := m.taskDb.List()
	if err != nil {
		log.Printf("Error getting task definitions: %v", err)
		return nil
	}
	return tasks
}

// --- Métodos relacionados con la salud de las tareas (Health Checks) ---
// (Estos métodos probablemente no cambian mucho, pero los incluyo para tener
//  el código completo).

func (m *Manager) DoHealthChecks() {
	for {
		log.Println("Performing task health check")
		m.doHealthChecks()
		log.Println("Task health checks completed")
		time.Sleep(60 * time.Second)
	}
}

func (m *Manager) doHealthChecks() {
	executions := m.GetTasks() // Obtener todas las TaskExecutions
	for _, exec := range executions {
		if exec.State == domain.Running { //Solo si esta corriendo
			err := m.checkTaskHealth(exec) //Pasamos la ejecucion
			if err != nil {
				// Si la tarea está fallando, *podrías* querer reiniciarla.
				// Pero, es mejor tener una política de reintentos separada
				// (por ejemplo, usando una cola de fallidos, un contador de reintentos, etc.).
				// Aqui *no* reiniciamos directamente.
				log.Printf("Task %s failed health check: %v", exec.ID, err)
			}
		} //Ya no necesitamos el else, solo se comprueba el estado Running.
	}
}

// checkTaskHealth realiza una verificación de salud para una TaskExecution.
func (m *Manager) checkTaskHealth(exec *domain.TaskExecution) error {
	// 1. Obtener la Task (para obtener la URL de health check)
	taskDef, err := m.taskDb.Get(exec.TaskID.String())
	if err != nil {
		return fmt.Errorf("failed to get task definition for health check: %w", err)
	}

	// 2.  Obtener la dirección del worker (del mapa taskWorkerMap)
	workerAddr, ok := m.taskWorkerMap[exec.ID]
	if !ok || workerAddr == "" {
		return fmt.Errorf("no worker found for task execution %s", exec.ID)
	}
	//Si no se definio endpoint, no se hace healthcheck.
	if exec.Status.Endpoint == nil || exec.Status.Endpoint.Ip == "" || exec.Status.Endpoint.Port == "" {
		log.Printf("No endpoint defined for task %s. Skipping health check.", exec.ID)
		return nil
	}

	// 3. Construir la URL de health check
	//    (Asumo que tienes un campo HealthCheckURL en Task.WorkerSpec)
	//    Si no, necesitarás construir la URL de otra forma (por ejemplo, usando
	//    la información de HostPorts, etc.).
	var url string
	//Si en la definicion de la tarea se definio un healthcheck, se usa ese.
	if taskDef.WorkerSpec.HealthCheck != "" {
		url = fmt.Sprintf("http://%s:%s%s", exec.Status.Endpoint.Ip, exec.Status.Endpoint.Port, taskDef.WorkerSpec.HealthCheck)
	} else {
		//Si no, se hace un ping a la direccion del worker
		url = fmt.Sprintf("http://%s:%s", exec.Status.Endpoint.Ip, exec.Status.Endpoint.Port) //Un simple ping
	}

	log.Printf("Calling health check for task %s: %s", exec.ID, url)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error connecting to health check %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check for task %s failed with status code %d", exec.ID, resp.StatusCode)
	}

	log.Printf("Task %s health check successful", exec.ID)
	return nil
}

// --- Otros métodos (posiblemente no necesarios) ---
// (Estos métodos probablemente no son necesarios en la nueva arquitectura,
//  pero los dejo comentados por si acaso).

// func (m *Manager) UpdatePoolStats() { /* ... */ } // Probablemente no es necesario
// func (m *Manager) stopTask(...) { /* ... */ } //Probablemente no es necesario, la tarea se detiene desde monitorTask

// ---  Funciones de utilidad ---

// getHostPort extrae un puerto expuesto de un nat.PortMap (para Docker).
func getHostPort(ports nat.PortMap) *string {
	for _, bindings := range ports {
		for _, binding := range bindings {
			return &binding.HostPort //Devolver el primero
		}
	}
	return nil
}
