package manager

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"k8s.io/apimachinery/pkg/util/rand"

	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/worker"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/service/scheduler"
	"fmt"
	"log"
	"time"
)

// HodeiApp manages task execution.
type HodeiApp struct {
	pendingTasksChan chan ports.TaskContext
	scheduler        ports.Scheduler
	worker           *worker.WorkerInstanceManager
	poolService      *ports.ResourcePoolService
	taskService      *ports.TaskService
	workerDefService *ports.WorkerDefinitionService
	taskExecService  *ports.TaskExecutionService
	generator        ports.IDGenerator
}

// NewHodeiApp creates a new HodeiApp instance.
func NewHodeiApp(
	schedulerType string,
	worker *worker.WorkerInstanceManager,
	poolService *ports.ResourcePoolService,
	taskService *ports.TaskService,
	workDefService *ports.WorkerDefinitionService,
	taskExecService *ports.TaskExecutionService,
	sizePendingsTask int,
	generator ports.IDGenerator,
) (ports.HodeiAppManager, error) {
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

	app := &HodeiApp{
		pendingTasksChan: make(chan ports.TaskContext, sizePendingsTask), // Buffer para 1000 tareas
		scheduler:        currentSheduler,
		worker:           worker, // Guardar la instancia del WorkerInstanceManager
		poolService:      poolService,
		taskService:      taskService,
		workerDefService: workDefService,
		taskExecService:  taskExecService,
		generator:        generator,
	}

	return app, nil
}

// AddTask añade una nueva Execution a la cola de pendientes.
func (m *HodeiApp) AddTask(request model.TaskExecutionRequest, ctx context.Context) (ports.TaskContext, error) {

	outputChan := make(chan model.ProcessOutput, 100)
	stateChan := make(chan model.TaskState, 10) // Canal para el estado
	errChan := make(chan error, 1)
	taskDef, err := (*m.taskService).GetTask(ctx, request.TaskID)
	if err != nil {
		return ports.TaskContext{}, fmt.Errorf("tarea no encontrada: %w", err)
	}
	log.Printf("Tarea %s recuperada para su ejecución", taskDef.Metadata.Name)

	workerDef, err := (*m.workerDefService).FindWorkerDefinitionByName(ctx, taskDef.Spec.WorkerDefinitionName)
	if err != nil {
		return ports.TaskContext{}, fmt.Errorf("definición de worker no encontrada: %w", err)
	}

	execution := model.TaskExecution{
		ID: m.generator.NewID(),
		Metadata: model.NewMetadata(
			taskDef.Metadata.Name+randString(5),
			taskDef.Metadata.Description,
		),
		Status: model.ExecutionStatus{
			StartTime: time.Now(),
			State:     model.Pending,
		},
		WorkerDef: workerDef,
	}
	(*m.taskExecService).CreateTaskExecution(ctx, &execution)

	taskContext := ports.TaskContext{
		Execution:  execution,
		OutputChan: outputChan,
		StateChan:  stateChan, // Asignar el canal de estado
		ErrChan:    errChan,   // Asignar el canal de errores
		Ctx:        ctx,
	}

	log.Printf("Tarea %s añadida al registro de tareas operables", taskDef.ID)

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

func (m *HodeiApp) SelectWorker(definition *model.WorkerDefinition) (*ports.ResourcePool, error) {

	// Obtener la lista de pools activos del ResourcePoolManager
	activePools := (*m.poolService).ListActivePools()
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
func (m *HodeiApp) ProcessTasks() {
	for taskID := range m.pendingTasksChan { // Escuchar el channel
		log.Printf("Procesando: %s", taskID)
		go m.processTask(taskID)
	}
}

// processTask maneja la lógica de una sola tarea:  selección, lanzamiento y actualización.
// processTask ahora delega la ejecución al WorkerInstanceManager.
func (m *HodeiApp) processTask(taskContext ports.TaskContext) {
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
	(*m.taskExecService).UpdateTaskExecutionStatus(taskContext.Ctx, taskContext.Execution.ID, taskContext.Execution.Status)
	log.Printf("Tarea %s procesada", taskDefID)
}

// DoHealthChecks realiza las verificaciones de salud para todas las tareas.
func (m *HodeiApp) DoHealthChecks() {
	m.doHealthChecks()
}

func (m *HodeiApp) doHealthChecks() {
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
func (m *HodeiApp) checkTaskHealth(exec *model.TaskExecution) error {
	// TODO: Implementar la lógica de verificación de salud real.
	//  Por ahora, solo imprimimos un mensaje.
	log.Printf("Verificando la salud de la tarea %s (Execution ID: %s)", exec.Task.ID, exec.ID)
	return nil
}

func (m *HodeiApp) StopTask(taskContext ports.TaskContext) error {
	log.Printf("Deteniendo la tarea %s", taskContext.Execution.ID)
	return m.worker.StopTask(taskContext)
}
