// worker/worker.go
package worker

import (
	"context"
	"fmt"
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
	prefixSetup  = "SETUP"
	prefixInfo   = "INFO"
	prefixError  = "ERROR"
	prefixWarn   = "WARN"
	prefixDebug  = "DEBUG"
	prefixStdout = "STDOUT"
	prefixStderr = "STDERR"
)

type TaskContext struct {
	Task       domain.Task
	outputChan chan<- *domain.ProcessOutput
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
func sendOutput(outputChan chan<- *domain.ProcessOutput, processID string, messageType string, message string, isError bool) {
	formattedMessage := fmt.Sprintf("[WORKER CLIENT][%s] %s", messageType, message)
	outputChan <- &domain.ProcessOutput{
		ProcessID: processID,
		Output:    formattedMessage,
		IsError:   isError,
	}
}

func (w *Worker) processTask(taskCtx TaskContext) {
	defer w.releaseSlot()
	defer close(taskCtx.outputChan)

	// Actualizar estado de la tarea
	task := taskCtx.Task
	task.State = domain.Running
	w.db.Put(task.ID.String(), task)
	sendOutput(taskCtx.outputChan, task.ID.String(), prefixInfo,
		fmt.Sprintf("Tarea en estado: %s", task.State), false)

	// Ejecutar la tarea
	workerInstance, err := w.workerFactory.Create(task)
	if err != nil {
		task.State = domain.Failed
		w.db.Put(task.ID.String(), task)
		sendOutput(taskCtx.outputChan, task.ID.String(), prefixError,
			fmt.Sprintf("Error creando worker instance: %v", err), true)
		return
	}

	w.activeWorkers.Store(task.ID.String(), workerInstance)
	defer w.activeWorkers.Delete(task.ID.String())

	endpoint, err := workerInstance.Start(taskCtx.ctx, taskCtx.outputChan)
	if err != nil {
		task.State = domain.Failed
		sendOutput(taskCtx.outputChan, task.ID.String(), prefixError,
			fmt.Sprintf("Error iniciando tarea: %v no se resolvio el enpoint", err), true)
		return
	}
	sendOutput(taskCtx.outputChan, task.ID.String(), prefixInfo,
		fmt.Sprintf("Tarea iniciada en %s", endpoint), false)

	// Canal para recibir actualizaciones de estado
	healthChan := make(chan *domain.ProcessHealthStatus, 10)
	defer close(healthChan)

	// Iniciar monitorización de salud
	if err := workerInstance.StartMonitoring(taskCtx.ctx, 5, healthChan); err != nil {
		task.State = domain.Failed
		sendOutput(taskCtx.outputChan, task.ID.String(), prefixError,
			fmt.Sprintf("Error iniciando monitorización: %v", err), true)
		return
	}

	// Ejecutar la tarea en una goroutine separada
	runErrChan := make(chan error, 1)
	runDoneChan := make(chan struct{})
	runCtx, runCancel := context.WithCancel(taskCtx.ctx)
	defer runCancel()

	go func() {
		defer close(runDoneChan)
		err := workerInstance.Run(runCtx, task, taskCtx.outputChan)
		runErrChan <- err
	}()

	// Monitorear el estado de la tarea
	var lastStatus domain.HealthStatus
	processTimeout := time.After(5 * time.Minute) // Timeout de seguridad
	taskCompleted := false

	for !taskCompleted {
		select {
		case <-runDoneChan:
			// El proceso ha terminado, esperamos el error del canal
			if err := <-runErrChan; err != nil {
				task.State = domain.Failed
				sendOutput(taskCtx.outputChan, task.ID.String(), prefixError,
					fmt.Sprintf("Error ejecutando tarea: %v", err), true)
				w.db.Put(task.ID.String(), task)
				taskCompleted = true
			} else {
				// Si no hay error, esperamos el estado FINISHED del health check
				sendOutput(taskCtx.outputChan, task.ID.String(), prefixInfo,
					"Proceso completado, esperando confirmación de estado", false)
			}

		case status := <-healthChan:
			if status == nil {
				continue
			}

			// Enviar el estado recibido al canal de salida
			sendOutput(taskCtx.outputChan, task.ID.String(), prefixDebug,
				fmt.Sprintf("Estado recibido: %v -> %s", status.Status, status.Message), false)

			// Solo procesar cambios de estado
			if status.Status != lastStatus {
				lastStatus = status.Status
				sendOutput(taskCtx.outputChan, task.ID.String(), prefixInfo,
					fmt.Sprintf("Cambio de estado: %v -> %s", status.Status, status.Message), false)

				switch status.Status {
				case domain.FINISHED:
					task.State = domain.Completed
					sendOutput(taskCtx.outputChan, task.ID.String(), prefixInfo,
						fmt.Sprintf("Tarea completada: %s", status.Message), false)

					// Esperar el tiempo configurado antes de parar el worker
					time.Sleep(w.workerFactory.GetStopDelay())
					if stopped, msg, err := workerInstance.Stop(taskCtx.ctx); err != nil {
						sendOutput(taskCtx.outputChan, task.ID.String(), prefixError,
							fmt.Sprintf("Error deteniendo worker: %v", err), true)
					} else if stopped {
						sendOutput(taskCtx.outputChan, task.ID.String(), prefixInfo,
							fmt.Sprintf("Worker detenido: %s", msg), false)
					}
					w.db.Put(task.ID.String(), task)
					taskCompleted = true

				case domain.ERROR:
					task.State = domain.Failed
					sendOutput(taskCtx.outputChan, task.ID.String(), prefixError,
						fmt.Sprintf("Error en la tarea: %s", status.Message), true)
					w.db.Put(task.ID.String(), task)
					taskCompleted = true

				case domain.STOPPED:
					task.State = domain.Stopped
					sendOutput(taskCtx.outputChan, task.ID.String(), prefixInfo,
						fmt.Sprintf("Tarea detenida: %s", status.Message), false)
					w.db.Put(task.ID.String(), task)
					taskCompleted = true
				}
			}

		case <-processTimeout:
			task.State = domain.Failed
			sendOutput(taskCtx.outputChan, task.ID.String(), prefixError,
				"Tarea cancelada por timeout", true)
			w.db.Put(task.ID.String(), task)
			taskCompleted = true

		case <-taskCtx.ctx.Done():
			task.State = domain.Stopped
			sendOutput(taskCtx.outputChan, task.ID.String(), prefixError,
				"Tarea cancelada por contexto", true)
			w.db.Put(task.ID.String(), task)
			taskCompleted = true
		}
	}

	// Enviar mensaje final antes de cerrar
	sendOutput(taskCtx.outputChan, task.ID.String(), prefixInfo,
		fmt.Sprintf("Tarea finalizada con estado: %s", task.State), false)
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

func (w *Worker) processPendingQueue() {
	for w.pendingQueue.Len() > 0 {
		if !w.acquireSlot() {
			break
		}
		taskCtx := w.pendingQueue.Dequeue().(TaskContext)
		go w.processTask(taskCtx)
	}
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

	sendOutput(outputChan, task.ID.String(), prefixInfo, "Tarea encolada exitosamente", false)
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
