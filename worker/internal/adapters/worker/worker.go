// worker/worker.go
package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"dev.rubentxu.devops-platform/worker/internal/domain"

	"dev.rubentxu.devops-platform/worker/internal/adapters/store"
	"dev.rubentxu.devops-platform/worker/internal/ports"
	"github.com/golang-collections/collections/queue"
)

// Constantes para los tipos de mensaje
const (
	TypeSetup  = "SETUP"
	TypeInfo   = "INFO"
	TypeWarn   = "WARN"
	TypeDebug  = "DEBUG"
	TypeStdout = "STDOUT"
	TypeStderr = "STDERR"
	TypeHealth = "HEALTH"
	TypeError  = "ERROR"
)

type TaskContext struct {
	Task       domain.Task
	outputChan chan *domain.ProcessOutput
	ctx        context.Context
}

type Worker struct {
	name          string
	db            store.Store[domain.Task]
	taskChan      chan TaskContext
	pendingQueue  queue.Queue
	pendingMutex  sync.Mutex
	workerFactory ports.WorkerFactory
	activeWorkers sync.Map
	configMutex   sync.RWMutex
	metrics       *domain.Metrics

	// Concurrency control
	maxConcurrent  int32
	currentTasks   int32
	scalingHistory []domain.ScalingEvent
	scalingChan    chan int
}

func NewWorker(name string, initialMaxConcurrent int, storeType string, workerFactory ports.WorkerFactory) *Worker {
	w := &Worker{
		name:          name,
		taskChan:      make(chan TaskContext, 100),
		pendingQueue:  *queue.New(),
		maxConcurrent: int32(initialMaxConcurrent),
		scalingChan:   make(chan int, 10),
		metrics:       &domain.Metrics{},
		workerFactory: workerFactory,
	}

	w.initStore(storeType)
	go w.taskDispatcher()
	go w.monitorResources()
	go w.scalingListener()

	return w
}

func (w *Worker) initStore(storeType string) {
	switch storeType {
	case "memory":
		w.db = store.NewInMemoryStore[domain.Task]()
	case "bolt":
		filename := fmt.Sprintf("%s_tasks.db", w.name)
		store, err := store.NewBoltDBStore[domain.Task](filename, 0600, "tasks")
		if err != nil {
			panic(err)
		}
		w.db = store
	}
}

func (w *Worker) taskDispatcher() {
	for taskCtx := range w.taskChan {
		if w.acquireSlot() {
			go w.processTask(taskCtx)
		} else {
			w.pendingQueue.Enqueue(taskCtx)
		}
	}
}

// sendOutput es una función auxiliar para enviar mensajes formateados al canal de salida
func sendOutput(outputChan chan<- *domain.ProcessOutput, processID string, messageType string, message string, isError bool, status domain.HealthStatus) {
	formattedMessage := fmt.Sprintf("[WORKER CLIENT] %s", message)
	outputChan <- &domain.ProcessOutput{
		ProcessID: processID,
		Output:    formattedMessage,
		IsError:   isError,
		Type:      messageType,
		Status:    status,
	}
	log.Printf("Output status: %s", status)

}

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

func (w *Worker) processTask(taskCtx TaskContext) {
	taskID := taskCtx.Task.ID.String()
	workerRemoved := make(chan struct{})

	defer func() {
		w.releaseSlot()
		w.activeWorkers.Delete(taskID)
		close(workerRemoved)
		// Intentar procesar la cola pendiente después de liberar un slot
		go w.processPendingQueue()
	}()
	defer close(taskCtx.outputChan)

	// Ejecutar la tarea
	workerInstance, err := w.workerFactory.Create(taskCtx.Task)
	if err != nil {
		sendOutput(taskCtx.outputChan, taskID, TypeError,
			fmt.Sprintf("Error creando worker instance: %v", err), true, domain.ERROR)
		return
	}

	// Defer para detener el worker al salir de processTask
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if stopped, msg, err := workerInstance.Stop(stopCtx); err != nil {
			log.Printf("[%s] Error deteniendo worker en defer: %v", taskID, err)
		} else if stopped {
			log.Printf("[%s] Worker detenido en defer: %s", taskID, msg)
		}
	}()

	// Inicializar la máquina de estados
	state := NewWorkerState(workerInstance, taskID)
	if err := state.SetState(domain.RUNNING); err != nil {
		log.Printf("[%s] Error estableciendo estado inicial: %v", taskID, err)
		return
	}

	w.activeWorkers.Store(taskID, workerInstance)

	endpoint, err := workerInstance.Start(taskCtx.ctx, taskCtx.outputChan)
	if err != nil {
		state.SetState(domain.ERROR)
		sendOutput(taskCtx.outputChan, taskID, TypeError,
			fmt.Sprintf("Error iniciando tarea: %v no se resolvio el endpoint", err), true, domain.ERROR)
		return
	}

	log.Printf("[%s] Tarea iniciada en %s", taskID, endpoint)

	// Ejecutar la tarea en una goroutine separada
	runErrChan := make(chan error, 1)
	runDoneChan := make(chan struct{})
	runCtx, runCancel := context.WithCancel(taskCtx.ctx)
	defer runCancel()

	go func() {
		defer close(runDoneChan)
		err := workerInstance.Run(runCtx, taskCtx.Task, taskCtx.outputChan)
		runErrChan <- err
	}()

	// Monitorear el estado de la tarea
	processTimeout := time.After(5 * time.Minute)
	taskCompleted := false

	for !taskCompleted {
		select {
		case <-runDoneChan:
			if err := <-runErrChan; err != nil {
				state.SetState(domain.ERROR)
				sendOutput(taskCtx.outputChan, taskID, TypeError,
					fmt.Sprintf("Error ejecutando tarea: %v", err), true, domain.ERROR)
				taskCompleted = true
			}

		case output := <-taskCtx.outputChan:
			if output.Type == TypeHealth {
				if err := state.SetState(output.Status); err != nil {
					log.Printf("[%s] Error en transición de estado: %v", taskID, err)
					continue
				}

				// Actualizar estado en BD y enviar notificación
				taskCtx.Task.State = convertHealthStatusToTaskState(output.Status)
				w.db.Put(taskID, taskCtx.Task)
				sendOutput(taskCtx.outputChan, taskID, TypeInfo,
					fmt.Sprintf("Estado actualizado: %s", output.Status), false, output.Status)

				if output.Status == domain.FINISHED || output.Status == domain.ERROR || output.Status == domain.STOPPED {
					taskCompleted = true
				}
			} else {
				// Reenviar otros tipos de mensajes
				taskCtx.outputChan <- output
			}

		case newState := <-state.stateChanged:
			log.Printf("[%s] Cambio de estado detectado: %v", taskID, newState)
			if newState == domain.FINISHED || newState == domain.ERROR || newState == domain.STOPPED {
				taskCompleted = true
			}

		case <-processTimeout:
			state.SetState(domain.ERROR)
			sendOutput(taskCtx.outputChan, taskID, TypeError,
				"Tarea cancelada por timeout", true, domain.ERROR)
			taskCompleted = true

		case <-taskCtx.ctx.Done():
			state.SetState(domain.STOPPED)
			sendOutput(taskCtx.outputChan, taskID, TypeError,
				"Tarea cancelada por contexto", true, domain.STOPPED)
			taskCompleted = true
		}
	}

	// Asegurarse de que el worker se elimine
	select {
	case <-workerRemoved:
		log.Printf("[%s] Worker eliminado correctamente", taskID)
	case <-time.After(5 * time.Second):
		log.Printf("[%s] Warning: No se pudo confirmar la eliminación del worker", taskID)
	}
}

func convertHealthStatusToTaskState(status domain.HealthStatus) domain.State {
	switch status {
	case domain.RUNNING:
		return domain.Running
	case domain.FINISHED:
		return domain.Completed
	case domain.ERROR:
		return domain.Failed
	case domain.STOPPED:
		return domain.Stopped
	default:
		return domain.Unknown
	}
}

func (w *Worker) processPendingQueue() {
	// Intentar procesar tantas tareas como slots disponibles haya
	for {
		w.pendingMutex.Lock()
		if w.pendingQueue.Len() == 0 {
			w.pendingMutex.Unlock()
			return
		}

		// Intentar adquirir un slot
		if !w.acquireSlot() {
			w.pendingMutex.Unlock()
			return
		}

		// Obtener la siguiente tarea y liberamos el lock inmediatamente
		taskCtx := w.pendingQueue.Dequeue().(TaskContext)
		w.pendingMutex.Unlock()

		// Procesar la tarea en una goroutine separada
		go func(ctx TaskContext) {
			w.processTask(ctx)
			// No necesitamos llamar a processPendingQueue aquí porque ya se llama en processTask
		}(taskCtx)
	}
}

func (w *Worker) retryStop(taskID string, instance ports.WorkerInstance) {
	retryCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for retries := 0; retries < 3; retries++ {
		if stopped, _, err := instance.Stop(retryCtx); err == nil && stopped {
			log.Printf("Worker %s detenido exitosamente después de %d reintentos", taskID, retries+1)
			return
		}
		time.Sleep(time.Second * time.Duration(retries+1))
	}
	log.Printf("No se pudo detener el worker %s después de 3 intentos", taskID)
}

func (w *Worker) acquireSlot() bool {
	current := atomic.LoadInt32(&w.currentTasks)
	max := atomic.LoadInt32(&w.maxConcurrent)
	if current < max {
		atomic.AddInt32(&w.currentTasks, 1)
		return true
	}
	return false
}

func (w *Worker) releaseSlot() {
	atomic.AddInt32(&w.currentTasks, -1)
}

// Control de concurrencia dinámico
func (w *Worker) scalingListener() {
	for newLimit := range w.scalingChan {
		w.configMutex.Lock()
		oldLimit := int(atomic.LoadInt32(&w.maxConcurrent))

		atomic.StoreInt32(&w.maxConcurrent, int32(newLimit))

		w.scalingHistory = append(w.scalingHistory, domain.ScalingEvent{
			Timestamp: time.Now(),
			OldLimit:  oldLimit,
			NewLimit:  newLimit,
			Reason:    "external adjustment",
		})

		if newLimit > oldLimit {
			w.processPendingQueue()
		}
		w.configMutex.Unlock()
	}
}

// API pública
func (w *Worker) AddTask(ctx context.Context, task domain.Task) (<-chan *domain.ProcessOutput, error) {
	outputChan := make(chan *domain.ProcessOutput, 100)
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	if err := w.db.Put(task.ID.String(), task); err != nil {
		close(outputChan)
		return nil, fmt.Errorf("error guardando tarea: %w", err)
	}

	w.taskChan <- TaskContext{
		Task:       task,
		outputChan: outputChan,
		ctx:        ctx,
	}

	sendOutput(outputChan, task.ID.String(), TypeInfo, "Tarea encolada exitosamente", false, domain.PENDING)
	return outputChan, nil
}

func (w *Worker) SetConcurrencyLimit(newLimit int) {
	if newLimit < 1 {
		newLimit = 1
	}
	if newLimit > 100 {
		newLimit = 100
	}
	w.scalingChan <- newLimit
}

func (w *Worker) GetStatus() domain.WorkerConfig {
	return domain.WorkerConfig{
		MaxConcurrentTasks: int(atomic.LoadInt32(&w.maxConcurrent)),
		CurrentTasks:       int(atomic.LoadInt32(&w.currentTasks)),
	}
}

// Monitorización de recursos
func (w *Worker) monitorResources() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		w.metrics.CPUUsage = getCPUUsage()
		w.metrics.MemoryUsage = getMemoryUsage()

		if w.metrics.CPUUsage > 80 && atomic.LoadInt32(&w.maxConcurrent) > 1 {
			w.SetConcurrencyLimit(int(atomic.LoadInt32(&w.maxConcurrent) - 1))
		}
	}
}

func getCPUUsage() float64    { return 45.0 }
func getMemoryUsage() float64 { return 60.0 }

// GetTasks retorna un slice con todas las tareas almacenadas.
func (w *Worker) GetTasks() ([]domain.Task, error) {
	taskList, err := w.db.List()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo lista de tareas: %w", err)
	}
	return taskList, nil
}

// GetTask retorna la tarea con el id dado.
func (w *Worker) GetTask(taskID string) (domain.Task, error) {
	t, err := w.db.Get(taskID)
	if err != nil {
		return domain.Task{}, fmt.Errorf("no se encontró la tarea con ID %s", taskID)
	}
	return t, nil
}

// StopTask localiza la tarea, cambia su estado y, de ser necesario, detiene el proceso subyacente.
func (w *Worker) StopTask(taskID string) error {
	task, err := w.db.Get(taskID)
	if err != nil {
		return fmt.Errorf("no se encontró la tarea: %w", err)
	}

	task.State = domain.Stopped
	if err := w.db.Put(taskID, task); err != nil {
		return fmt.Errorf("error actualizando estado a Stopped: %w", err)
	}
	return nil
}
