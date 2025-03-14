package worker

import (
	"context"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"fmt"
	"log"
	"sync"

	"time"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
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

var _ ports.WorkerInstanceManager = (*WorkerInstanceManagerImpl)(nil)

type workerOperation struct {
	op     string
	taskID string
	worker ports.WorkerInstance
	result chan error
}

type WorkerInstanceManagerImpl struct {
	name            string
	taskExecService *ports.TaskExecutionService
	workerFactory   ports.WorkerFactory

	// Channels for Execution management
	taskQueue     chan ports.TaskContext
	activeWorkers sync.Map // Cambiado a sync.Map para evitar race conditions
	workerChan    chan workerOperation

	// Concurrency control channels
	concurrencyLimitChan chan int
	workerSlots          chan struct{}
	//metrics              chan model.Metrics

	// TaskState
	maxConcurrent int32 // Cambiado a int32 para uso atómico
	//scalingHistory []model.ScalingEvent

	// Control de shutdown
	shutdown chan struct{}
	wg       sync.WaitGroup
}

func NewWorker(name string, initialMaxConcurrent int, workerFactory ports.WorkerFactory, taskExecService *ports.TaskExecutionService) ports.WorkerInstanceManager {
	workerInstanceManager := &WorkerInstanceManagerImpl{
		name:                 name,
		workerFactory:        workerFactory,
		taskQueue:            make(chan ports.TaskContext, 100),
		workerChan:           make(chan workerOperation, 10),
		concurrencyLimitChan: make(chan int),
		workerSlots:          make(chan struct{}, initialMaxConcurrent),
		taskExecService:      taskExecService,
		//metrics:              make(chan model.Metrics, 1),
		activeWorkers: sync.Map{},
		maxConcurrent: int32(initialMaxConcurrent),
		shutdown:      make(chan struct{}),
	}

	// Initialize slots
	for i := 0; i < initialMaxConcurrent; i++ {
		workerInstanceManager.workerSlots <- struct{}{}
	}

	workerInstanceManager.wg.Add(2) // Para taskDispatcher y workerManager
	go workerInstanceManager.taskDispatcher()
	go workerInstanceManager.workerManager()

	return workerInstanceManager
}

func (w *WorkerInstanceManagerImpl) Stop() error {
	close(w.shutdown)
	w.wg.Wait()
	return nil
}

func (w *WorkerInstanceManagerImpl) taskDispatcher() {
	defer w.wg.Done()
	pendingTasks := make([]ports.TaskContext, 0)

	for {
		select {
		case <-w.shutdown:
			return
		case task := <-w.taskQueue:
			select {
			case <-w.workerSlots:
				// Slot available, process Execution
				go w.processTask(task)
			default:
				// No slots available, add to pending
				pendingTasks = append(pendingTasks, task)
			}

		case <-w.workerSlots:
			// A slot became available, process pending Execution if any
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
func sendOutput(outputChan chan<- model.ProcessOutput, processID string, messageType string, message string, isError bool, status model.HealthStatus) {
	formattedMessage := fmt.Sprintf("[WORKER CLIENT] %s", message)
	outputChan <- model.ProcessOutput{
		ProcessID: processID,
		Output:    formattedMessage,
		IsError:   isError,
		Type:      messageType,
		Status:    status,
	}
	log.Printf("Output status: %s", status)

}

func (w *WorkerInstanceManagerImpl) processTask(op ports.TaskContext) {
	taskID := op.Execution.ID.String()
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

	workerInstance, err := w.workerFactory.Create(op.Execution, op.Client)
	if err != nil {
		sendOutput(op.OutputChan, taskID, TypeError,
			fmt.Sprintf("Error creating worker instance: %v", err), true, model.ERROR)
		op.ErrChan <- err
		return
	}

	// Registrar worker con sync.Map
	w.activeWorkers.Store(taskID, workerInstance)
	defer w.activeWorkers.Delete(taskID)

	// Notificar que la tarea está iniciando
	sendOutput(op.OutputChan, taskID, TypeInfo,
		"Iniciando preparación del worker...", false, model.PENDING)

	// Notificar que el worker se ha creado
	sendOutput(op.OutputChan, taskID, TypeInfo,
		"WorkerInstanceManagerImpl creado, iniciando configuración...", false, model.PENDING)

	// Defer para detener el worker al salir de processTask
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if stopped, msg, err := workerInstance.Stop(stopCtx); err != nil {
			log.Printf("[%s] Error deteniendo worker en defer: %v", taskID, err)
			sendOutput(op.OutputChan, taskID, TypeWarn,
				fmt.Sprintf("Error deteniendo worker: %v", err), true, model.ERROR)
		} else if stopped {
			log.Printf("[%s] WorkerInstanceManagerImpl detenido en defer: %s", taskID, msg)
			sendOutput(op.OutputChan, taskID, TypeInfo,
				"WorkerInstanceManagerImpl detenido correctamente", false, model.DONE)
		}
	}()

	// Inicializar la máquina de estados
	state := NewWorkerState(workerInstance, taskID)
	if err := state.SetState(model.RUNNING); err != nil {
		log.Printf("[%s] Error estableciendo estado inicial: %v", taskID, err)
		sendOutput(op.OutputChan, taskID, TypeError,
			fmt.Sprintf("Error estableciendo estado inicial: %v", err), true, model.ERROR)
		return
	}

	w.activeWorkers.Store(taskID, workerInstance)

	// Notificar que el worker está iniciando
	sendOutput(op.OutputChan, taskID, TypeInfo,
		"Iniciando worker...", false, model.RUNNING)
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
      - SERVER_CERT_PATH=/certs/remote_worker-cert.pem
      - SERVER_KEY_PATH=/certs/remote_worker-key.pem
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
		state.SetState(model.ERROR)
		sendOutput(op.OutputChan, taskID, TypeError,
			fmt.Sprintf("Error iniciando worker: %v", err), true, model.ERROR)
		return
	}

	log.Printf("[%s] Tarea iniciada en %s", taskID, endpoint)
	sendOutput(op.OutputChan, taskID, TypeInfo,
		fmt.Sprintf("WorkerInstanceManagerImpl iniciado en %s", endpoint), false, model.RUNNING)

	// Ejecutar la tarea en una goroutine separada
	runErrChan := make(chan error, 1)
	runDoneChan := make(chan struct{})
	runCtx, runCancel := context.WithCancel(op.Ctx)
	defer runCancel()

	go func() {
		defer close(runDoneChan)
		err := workerInstance.Run(runCtx, op.Execution, op.OutputChan)
		runErrChan <- err
	}()

	// Monitorear el estado de la tarea
	processTimeout := time.After(5 * time.Minute)
	taskCompleted := false

	for !taskCompleted {
		select {
		case <-runDoneChan:
			if err := <-runErrChan; err != nil {
				state.SetState(model.ERROR)
				sendOutput(op.OutputChan, taskID, TypeError,
					fmt.Sprintf("Error ejecutando tarea: %v", err), true, model.ERROR)
				taskCompleted = true
			}

		case output := <-op.OutputChan:
			if output.Type == TypeHealth {
				if err := state.SetState(output.Status); err != nil {
					log.Printf("[%s] Error en transición de estado: %v", taskID, err)
					continue
				}

				// Actualizar estado en BD y enviar notificación
				op.Execution.Status.State = convertHealthStatusToTaskState(output.Status)
				(*w.taskExecService).UpdateTaskExecutionStatus(op.Ctx, op.Execution.ID, op.Execution.Status)

				// Solo enviar notificación si el estado ha cambiado significativamente
				if output.Status == model.FINISHED || output.Status == model.ERROR ||
					output.Status == model.STOPPED || output.Status == model.HEALTHY {
					sendOutput(op.OutputChan, taskID, TypeInfo,
						fmt.Sprintf("Estado actualizado: %s", output.Status), false, output.Status)
				}

				if output.Status == model.FINISHED || output.Status == model.ERROR ||
					output.Status == model.STOPPED {
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
			if newState == model.FINISHED || newState == model.ERROR || newState == model.STOPPED {
				taskCompleted = true
			}

		case <-processTimeout:
			state.SetState(model.ERROR)
			sendOutput(op.OutputChan, taskID, TypeError,
				"Tarea cancelada por timeout", true, model.ERROR)
			taskCompleted = true

		case <-op.Ctx.Done():
			state.SetState(model.STOPPED)
			sendOutput(op.OutputChan, taskID, TypeError,
				"Tarea cancelada por contexto", true, model.STOPPED)
			taskCompleted = true
		}
	}

	// Asegurarse de que el worker se elimine
	select {
	case <-doneChan:
		log.Printf("[%s] WorkerInstanceManagerImpl eliminado correctamente", taskID)
		sendOutput(op.OutputChan, taskID, TypeInfo,
			"WorkerInstanceManagerImpl eliminado correctamente", false, model.DONE)
	case <-time.After(5 * time.Second):
		log.Printf("[%s] Warning: No se pudo confirmar la eliminación del worker", taskID)
		sendOutput(op.OutputChan, taskID, TypeWarn,
			"No se pudo confirmar la eliminación del worker", false, model.ERROR)
	}
}

func convertHealthStatusToTaskState(status model.HealthStatus) model.TaskState {
	switch status {
	case model.RUNNING:
		return model.Running
	case model.FINISHED:
		return model.Completed
	case model.ERROR:
		return model.Failed
	case model.STOPPED:
		return model.Stopped
	case model.DONE:
		return model.Done

	default:
		return model.Unknown
	}
}

func (w *WorkerInstanceManagerImpl) workerManager() {
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

func (w *WorkerInstanceManagerImpl) AddTask(taskContext ports.TaskContext) error {
	w.taskQueue <- taskContext

	sendOutput(taskContext.OutputChan, taskContext.Execution.ID.String(), TypeInfo, "Execution queued successfully", false, model.PENDING)
	return nil
}

func (w *WorkerInstanceManagerImpl) SetConcurrencyLimit(newLimit int) {
	if newLimit < 1 {
		newLimit = 1
	}
	if newLimit > 100 {
		newLimit = 100
	}
	w.concurrencyLimitChan <- newLimit
}

//func (w *WorkerInstanceManagerImpl) GetStatus() model.WorkerConfig {
//	return model.WorkerConfig{
//		MaxConcurrentTasks: int(atomic.LoadInt32(&w.maxConcurrent)),
//	}
//}

func getCPUUsage() float64    { return 45.0 }
func getMemoryUsage() float64 { return 60.0 }

// StopTask localiza la tarea, cambia su estado y, de ser necesario, detiene el proceso subyacente.
func (w *WorkerInstanceManagerImpl) StopTask(taskContext ports.TaskContext) error {
	// TODO: Implementar
	return nil
}
