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

func (w *Worker) processTask(taskCtx TaskContext) {
	defer w.releaseSlot()

	// No cerramos el canal aquí, lo haremos después de enviar el último mensaje
	// defer close(taskCtx.outputChan)

	// Actualizar estado de la tarea
	task := taskCtx.Task
	task.State = domain.Running
	w.db.Put(task.ID.String(), task)

	// Ejecutar la tarea
	workerInstance, err := w.workerFactory.Create(task)
	if err != nil {
		task.State = domain.Failed
		w.db.Put(task.ID.String(), task)
		taskCtx.outputChan <- &domain.ProcessOutput{
			Output:  fmt.Sprintf("Error creando worker instance: %v", err),
			IsError: true,
		}
		close(taskCtx.outputChan)
		return
	}

	w.activeWorkers.Store(task.ID.String(), workerInstance)
	defer w.activeWorkers.Delete(task.ID.String())

	endpoint, err := workerInstance.Start(taskCtx.ctx, taskCtx.outputChan)
	if err != nil {
		task.State = domain.Failed
		taskCtx.outputChan <- &domain.ProcessOutput{
			Output:  fmt.Sprintf("Error iniciando tarea: %v no se resolvio el enpoint", err),
			IsError: true,
		}
		close(taskCtx.outputChan)
		return
	}
	taskCtx.outputChan <- &domain.ProcessOutput{
		Output:  fmt.Sprintf("Tarea iniciada en %s", endpoint),
		IsError: false,
	}

	// Ejecutar y monitorear
	if err := workerInstance.Run(taskCtx.ctx, task, taskCtx.outputChan); err != nil {
		task.State = domain.Failed
		taskCtx.outputChan <- &domain.ProcessOutput{
			Output:  fmt.Sprintf("Error ejecutando tarea: %v", err),
			IsError: true,
		}
	} else {
		task.State = domain.Completed
		taskCtx.outputChan <- &domain.ProcessOutput{
			Output:  "Tarea completada exitosamente",
			IsError: false,
		}
	}

	// Actualizar estado final y cerrar el canal
	w.db.Put(task.ID.String(), task)
	close(taskCtx.outputChan)

	// Procesar siguiente tarea pendiente
	w.processPendingQueue()
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
	outputChan := make(chan *domain.ProcessOutput, 100) // Buffer para evitar bloqueos
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	// Guardar la tarea en la base de datos
	if err := w.db.Put(task.ID.String(), task); err != nil {
		close(outputChan)
		return nil, fmt.Errorf("error guardando tarea: %w", err)
	}

	w.taskChan <- TaskContext{
		Task:       task,
		outputChan: outputChan,
		ctx:        ctx,
	}
	outputChan <- &domain.ProcessOutput{
		Output:    "Tarea encolada exitosamente",
		IsError:   false,
		ProcessID: task.ID.String(),
	}
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
