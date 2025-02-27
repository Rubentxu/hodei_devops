// worker/worker.go
package worker

import (
	"context"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/store"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"dev.rubentxu.devops-platform/orchestrator/internal/domain"

	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
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
}

type workerOperation struct {
	op     string
	taskID string
	worker ports.WorkerInstance
	result chan error
}

type Worker struct {
	name          string
	db            ports.Store[domain.TaskExecution]
	workerFactory ports.WorkerFactory

	// Channels for Task management
	taskQueue     chan ports.TaskOperation
	activeWorkers sync.Map // Cambiado a sync.Map para evitar race conditions
	workerChan    chan workerOperation

	// Concurrency control channels
	concurrencyLimitChan chan int
	workerSlots          chan struct{}
	metrics              chan domain.Metrics

	// State
	maxConcurrent  int32 // Cambiado a int32 para uso atómico
	scalingHistory []domain.ScalingEvent

	// Control de shutdown
	shutdown chan struct{}
	wg       sync.WaitGroup
}

func NewWorker(name string, initialMaxConcurrent int, workerFactory ports.WorkerFactory) *Worker {
	w := &Worker{
		name:                 name,
		workerFactory:        workerFactory,
		taskQueue:            make(chan ports.TaskOperation, 100),
		workerChan:           make(chan workerOperation, 10),
		concurrencyLimitChan: make(chan int),
		workerSlots:          make(chan struct{}, initialMaxConcurrent),
		metrics:              make(chan domain.Metrics, 1),
		activeWorkers:        sync.Map{},
		maxConcurrent:        int32(initialMaxConcurrent),
		shutdown:             make(chan struct{}),
	}

	// Initialize slots
	for i := 0; i < initialMaxConcurrent; i++ {
		w.workerSlots <- struct{}{}
	}

	w.db = store.NewCacheStore[domain.TaskExecution]()

	w.wg.Add(2) // Para taskDispatcher y workerManager
	go w.taskDispatcher()
	go w.workerManager()

	return w
}

func (w *Worker) Stop() {
	close(w.shutdown)
	w.wg.Wait()
}

func (w *Worker) taskDispatcher() {
	defer w.wg.Done()
	pendingTasks := make([]ports.TaskOperation, 0)

	for {
		select {
		case <-w.shutdown:
			return
		case task := <-w.taskQueue:
			select {
			case <-w.workerSlots:
				// Slot available, process Task
				go w.processTask(task)
			default:
				// No slots available, add to pending
				pendingTasks = append(pendingTasks, task)
			}

		case <-w.workerSlots:
			// A slot became available, process pending Task if any
			if len(pendingTasks) > 0 {
				task := pendingTasks[0]
				pendingTasks = pendingTasks[1:]
				go w.processTask(task)
			} else {
				// Return the slot if no pending tasks
				w.workerSlots <- struct{}{}
			}
		}
	}
}

// sendOutput es una función auxiliar para enviar mensajes formateados al canal de salida
func sendOutput(outputChan chan<- domain.ProcessOutput, processID string, messageType string, message string, isError bool, status domain.HealthStatus) {
	formattedMessage := fmt.Sprintf("[WORKER CLIENT] %s", message)
	outputChan <- domain.ProcessOutput{
		ProcessID: processID,
		Output:    formattedMessage,
		IsError:   isError,
		Type:      messageType,
		Status:    status,
	}
	log.Printf("Output status: %s", status)

}

func (w *Worker) processTask(op ports.TaskOperation) {
	taskID := op.Task.ID.String()
	doneChan := make(chan struct{})
	defer close(doneChan)

	// Crear un contexto con timeout para toda la operación
	ctx, cancel := context.WithTimeout(op.Ctx, 30*time.Minute)
	defer cancel()

	// Release slot when done
	defer func() { w.workerSlots <- struct{}{} }()

	// Create worker instance with timeout
	_, instanceCancel := context.WithTimeout(ctx, 1*time.Minute)
	defer instanceCancel()

	workerInstance, err := w.workerFactory.Create(op.Task)
	if err != nil {
		sendOutput(op.OutputChan, taskID, TypeError,
			fmt.Sprintf("Error creating worker instance: %v", err), true, domain.ERROR)
		op.ErrChan <- err
		return
	}

	// Registrar worker con sync.Map
	w.activeWorkers.Store(taskID, workerInstance)
	defer w.activeWorkers.Delete(taskID)

	// Notificar que la tarea está iniciando
	sendOutput(op.OutputChan, taskID, TypeInfo,
		"Iniciando preparación del worker...", false, domain.PENDING)

	// Notificar que el worker se ha creado
	sendOutput(op.OutputChan, taskID, TypeInfo,
		"Worker creado, iniciando configuración...", false, domain.PENDING)

	// Defer para detener el worker al salir de processTask
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if stopped, msg, err := workerInstance.Stop(stopCtx); err != nil {
			log.Printf("[%s] Error deteniendo worker en defer: %v", taskID, err)
			sendOutput(op.OutputChan, taskID, TypeWarn,
				fmt.Sprintf("Error deteniendo worker: %v", err), true, domain.ERROR)
		} else if stopped {
			log.Printf("[%s] Worker detenido en defer: %s", taskID, msg)
			sendOutput(op.OutputChan, taskID, TypeInfo,
				"Worker detenido correctamente", false, domain.DONE)
		}
	}()

	// Inicializar la máquina de estados
	state := NewWorkerState(workerInstance, taskID)
	if err := state.SetState(domain.RUNNING); err != nil {
		log.Printf("[%s] Error estableciendo estado inicial: %v", taskID, err)
		sendOutput(op.OutputChan, taskID, TypeError,
			fmt.Sprintf("Error estableciendo estado inicial: %v", err), true, domain.ERROR)
		return
	}

	w.activeWorkers.Store(taskID, workerInstance)

	// Notificar que el worker está iniciando
	sendOutput(op.OutputChan, taskID, TypeInfo,
		"Iniciando worker...", false, domain.RUNNING)
	const defaultComposeTemplate = `
name: ${PROJECT_NAME} 
services:
  worker:
    image: "${MY_IMAGE}"
    container_name: "${PROJECT_NAME}-worker"
    environment:
      - PROJECT_NAME=${PROJECT_NAME}
      - WORKER_NAME=${PROJECT_NAME}-worker
      - WORKER_HOST=worker-${PROJECT_NAME}
      - ENV=${APP_ENV}
      - JWT_SECRET=${JWT_SECRET}
      - SERVER_CERT_PATH=/certs/remote_process-cert.pem
      - SERVER_KEY_PATH=/certs/remote_process-key.pem
      - CA_CERT_PATH=/certs/ca-cert.pem
      - APPLICATION_PORT=50051
    ports:
      - ":50051"
    networks:
      - workers

networks:
  workers:
    driver: bridge
`

	endpoint, err := workerInstance.Start(op.Ctx, defaultComposeTemplate, op.OutputChan)
	if err != nil {
		state.SetState(domain.ERROR)
		sendOutput(op.OutputChan, taskID, TypeError,
			fmt.Sprintf("Error iniciando worker: %v", err), true, domain.ERROR)
		return
	}

	log.Printf("[%s] Tarea iniciada en %s", taskID, endpoint)
	sendOutput(op.OutputChan, taskID, TypeInfo,
		fmt.Sprintf("Worker iniciado en %s", endpoint), false, domain.RUNNING)

	// Ejecutar la tarea en una goroutine separada
	runErrChan := make(chan error, 1)
	runDoneChan := make(chan struct{})
	runCtx, runCancel := context.WithCancel(op.Ctx)
	defer runCancel()

	go func() {
		defer close(runDoneChan)
		err := workerInstance.Run(runCtx, op.Task, op.OutputChan)
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
				sendOutput(op.OutputChan, taskID, TypeError,
					fmt.Sprintf("Error ejecutando tarea: %v", err), true, domain.ERROR)
				taskCompleted = true
			}

		case output := <-op.OutputChan:
			if output.Type == TypeHealth {
				if err := state.SetState(output.Status); err != nil {
					log.Printf("[%s] Error en transición de estado: %v", taskID, err)
					continue
				}

				// Actualizar estado en BD y enviar notificación
				op.Task.State = convertHealthStatusToTaskState(output.Status)
				w.db.Put(taskID, op.Task)

				// Solo enviar notificación si el estado ha cambiado significativamente
				if output.Status == domain.FINISHED || output.Status == domain.ERROR ||
					output.Status == domain.STOPPED || output.Status == domain.HEALTHY {
					sendOutput(op.OutputChan, taskID, TypeInfo,
						fmt.Sprintf("Estado actualizado: %s", output.Status), false, output.Status)
				}

				if output.Status == domain.FINISHED || output.Status == domain.ERROR ||
					output.Status == domain.STOPPED {
					taskCompleted = true
				}
			} else {
				// Enviar el mensaje al websocket sin reenviarlo al canal
				select {
				case op.OutputChan <- output:
					// Mensaje enviado exitosamente
				default:
					// Canal lleno, loguear y continuar
					log.Printf("[%s] Canal de salida lleno, mensaje descartado: %s", taskID, output.Output)
				}
			}

		case newState := <-state.stateChanged:
			log.Printf("[%s] Cambio de estado detectado: %v", taskID, newState)
			if newState == domain.FINISHED || newState == domain.ERROR || newState == domain.STOPPED {
				taskCompleted = true
			}

		case <-processTimeout:
			state.SetState(domain.ERROR)
			sendOutput(op.OutputChan, taskID, TypeError,
				"Tarea cancelada por timeout", true, domain.ERROR)
			taskCompleted = true

		case <-op.Ctx.Done():
			state.SetState(domain.STOPPED)
			sendOutput(op.OutputChan, taskID, TypeError,
				"Tarea cancelada por contexto", true, domain.STOPPED)
			taskCompleted = true
		}
	}

	// Asegurarse de que el worker se elimine
	select {
	case <-doneChan:
		log.Printf("[%s] Worker eliminado correctamente", taskID)
		sendOutput(op.OutputChan, taskID, TypeInfo,
			"Worker eliminado correctamente", false, domain.DONE)
	case <-time.After(5 * time.Second):
		log.Printf("[%s] Warning: No se pudo confirmar la eliminación del worker", taskID)
		sendOutput(op.OutputChan, taskID, TypeWarn,
			"No se pudo confirmar la eliminación del worker", false, domain.ERROR)
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
	case domain.DONE:
		return domain.Done

	default:
		return domain.Unknown
	}
}

func (w *Worker) workerManager() {
	defer w.wg.Done()
	for {
		select {
		case <-w.shutdown:
			return
		case op := <-w.workerChan:
			switch op.op {
			case "add":
				w.activeWorkers.Store(op.taskID, op.worker)
				op.result <- nil
			case "remove":
				w.activeWorkers.Delete(op.taskID)
				op.result <- nil
			case "get":
				if worker, exists := w.activeWorkers.Load(op.taskID); !exists {
					op.result <- fmt.Errorf("worker not found: %s", op.taskID)
				} else {
					op.worker = worker.(ports.WorkerInstance)
					op.result <- nil
				}
			}
		}
	}
}

func (w *Worker) AddTask(operation ports.TaskOperation) error {
	w.taskQueue <- operation

	sendOutput(operation.OutputChan, operation.Task.ID.String(), TypeInfo, "Task queued successfully", false, domain.PENDING)
	return nil
}

func (w *Worker) SetConcurrencyLimit(newLimit int) {
	if newLimit < 1 {
		newLimit = 1
	}
	if newLimit > 100 {
		newLimit = 100
	}
	w.concurrencyLimitChan <- newLimit
}

func (w *Worker) GetStatus() domain.WorkerConfig {
	return domain.WorkerConfig{
		MaxConcurrentTasks: int(atomic.LoadInt32(&w.maxConcurrent)),
	}
}

func getCPUUsage() float64    { return 45.0 }
func getMemoryUsage() float64 { return 60.0 }

// GetTasks retorna un slice con todas las tareas almacenadas.
func (w *Worker) GetTasks() ([]domain.TaskExecution, error) {
	taskList, err := w.db.List()
	if err != nil {
		return nil, fmt.Errorf("[WORKER CLIENT] error obteniendo lista de tareas: %w", err)
	}
	return taskList, nil
}

// GetTask retorna la tarea con el id dado.
func (w *Worker) GetTask(taskID string) (domain.TaskExecution, error) {
	t, err := w.db.Get(taskID)
	if err != nil {
		return domain.TaskExecution{}, fmt.Errorf("[WORKER CLIENT] no se encontró la tarea con ID %s", taskID)
	}
	return t, nil
}

// StopTask localiza la tarea, cambia su estado y, de ser necesario, detiene el proceso subyacente.
func (w *Worker) StopTask(taskID string) error {
	task, err := w.db.Get(taskID)
	if err != nil {
		return fmt.Errorf("[WORKER CLIENT] no se encontró la tarea: %w", err)
	}

	task.State = domain.Stopped
	if err := w.db.Put(taskID, task); err != nil {
		return fmt.Errorf("[WORKER CLIENT] error actualizando estado a Stopped: %w", err)
	}
	return nil
}
