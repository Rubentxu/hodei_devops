package worker

import (
	"context"
	"dev.rubentxu.devops-platform/worker/internal/adapters/store"
	"dev.rubentxu.devops-platform/worker/internal/adapters/worker_instance"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"log"
	"time"

	"dev.rubentxu.devops-platform/worker/internal/ports"
	"google.golang.org/grpc/benchmark/stats"

	"github.com/golang-collections/collections/queue"
)

type TaskContext struct {
	Task       domain.Task
	outputChan chan<- *domain.ProcessOutput
	goContext  context.Context
}

type Worker struct {
	Name  string
	Queue queue.Queue
	Db    store.Store[domain.Task]
	//data        map[uuid.UUID]*task.Task
	Stats                 *stats.Stats
	TaskCount             int
	WorkerInstanceFactory ports.WorkerInstanceFactory
	workerInstances       map[string]ports.WorkerInstance
}

func New(name string, taskDbType string) *Worker {
	w := Worker{
		Name:                  name,
		Queue:                 *queue.New(),
		WorkerInstanceFactory: worker_instance.NewWorkerInstanceFactory(),
		workerInstances:       make(map[string]ports.WorkerInstance),
	}

	var s store.Store[domain.Task]
	var err error
	switch taskDbType {
	case "memory":
		s = store.NewInMemoryStore[domain.Task]()
	case "persistent":
		filename := fmt.Sprintf("%s_tasks.db", name)
		s, err = store.NewBoltDBStore[domain.Task](filename, 0600, "tasks")
	}
	if err != nil {
		log.Printf("eunable to create new task store: %v", err)
	}
	w.Db = s
	return &w
}

func (w *Worker) GetTasks() ([]*domain.Task, error) {
	taskListInterface, err := w.Db.List()
	if err != nil {
		return nil, err
	}

	taskList := make([]*domain.Task, 0)
	for _, t := range taskListInterface {
		taskList = append(taskList, &t)
	}
	return taskList, nil
}

//func (w *Worker) CollectStats() {
//	for {
//		log.Println("Collecting stats")
//		w.Stats = stats.GetStats()
//		w.TaskCount = w.Stats.TaskCount
//		time.Sleep(15 * time.Second)
//	}
//}

func generateWorkerID(taskName string) string {
	hash := uuid.New()
	return fmt.Sprintf("%s-%s", taskName, hash.String())
}

func (w *Worker) AddTask(t domain.Task) chan *domain.ProcessOutput {
	outputChan := make(chan *domain.ProcessOutput)
	workerID := generateWorkerID(t.Name)
	t.Status.WorkerID = &domain.WorkerID{ID: workerID}
	taskContext := TaskContext{Task: t, outputChan: outputChan, goContext: context.Background()}
	w.Queue.Enqueue(taskContext)
	log.Printf("Task %v added to queue\n with workerID: %v\n", t.ID, workerID)
	return outputChan
}

func (w *Worker) RunTasks() {
	for {
		if w.Queue.Len() != 0 {
			result := w.runTask()
			if result.Error != nil {
				log.Printf("Error running task: %v\n", result.Error)
			}
		} else {
			log.Printf("No tasks to process currently.\n")
		}
		log.Println("Sleeping for 4 seconds.")
		time.Sleep(4 * time.Second)
	}

}

func (w *Worker) runTask() domain.TaskResult {
	t := w.Queue.Dequeue()
	if t == nil {
		log.Println("[worker] No tasks in the queue")
		return domain.TaskResult{Error: nil}
	}

	taskContext := t.(TaskContext)
	fmt.Printf("[worker] Found task in queue: %v:\n", taskContext)

	err := w.Db.Put(taskContext.Task.ID.String(), taskContext.Task)
	if err != nil {
		msg := fmt.Errorf("error storing task %s: %v", taskContext.Task.ID.String(), err)
		log.Println(msg)
		return domain.TaskResult{Error: msg}
	}

	taskPersisted, err := w.Db.Get(taskContext.Task.ID.String())
	if err != nil {
		msg := fmt.Errorf("error getting task %s from database: %v", taskContext.Task.ID.String(), err)
		log.Println(msg)
		return domain.TaskResult{Error: msg}
	}

	log.Printf("[worker] Task persisted: %v\n", taskPersisted)

	if taskPersisted.State == domain.Completed {
		return w.StopTask(taskPersisted)
	}

	var taskResult domain.TaskResult
	if domain.ValidStateTransition(taskPersisted.State, taskContext.Task.State) {
		switch taskContext.Task.State {
		case domain.Scheduled:
			log.Printf("[worker] Task %v is scheduled\n with workerID: %v\n", taskContext.Task.ID, taskContext.Task.Status.WorkerID)
			if taskContext.Task.Status.WorkerID != nil && taskContext.Task.Status.WorkerID.ID != "" {
				taskResult = w.StopTask(taskContext.Task)
				if taskResult.Error != nil {
					log.Printf("%v\n", taskResult.Error)
				}
			}
			log.Printf("[worker] Task %v is preparing to start\n", taskContext.Task.ID)
			taskResult = w.StartTask(taskContext)
			log.Printf("[worker] Task %v has started\n", taskContext.Task.ID)
		default:
			log.Printf("This is a mistake. taskPersisted: %v, taskContext: %v\n", taskPersisted, taskContext)
			taskResult.Error = errors.New("We should not get here")
		}
	} else {
		err := fmt.Errorf("Invalid transition from %v to %v", taskPersisted.State, taskContext.Task.State)
		taskResult.Error = err
		log.Println(err)
		return taskResult
	}
	return taskResult
}

func (w *Worker) StartTask(t TaskContext) domain.TaskResult {
	log.Printf("[worker] Starting task: %v\n", t)

	inst, err := w.WorkerInstanceFactory.Create(t.Task.WorkerInstanceSpec)
	if err != nil {
		return domain.TaskResult{Error: fmt.Errorf("error creating worker instance: %v", err)}
	}
	w.workerInstances[t.Task.Status.WorkerID.ID] = inst

	workerID, err := inst.Start(t.goContext)
	if err != nil {
		return domain.TaskResult{Error: fmt.Errorf("error starting worker instance: %v", err)}
	}
	t.Task.Status.WorkerID = workerID

	result := inst.Run(t.goContext, t.Task, t.outputChan)
	if result.Error != nil {
		log.Printf("Err running task %v: %v\n", t.Task.ID, result.Error)
		t.Task.State = domain.Failed
		_ = w.Db.Put(t.Task.ID.String(), t.Task)
		return result
	}

	t.Task.State = domain.Running
	_ = w.Db.Put(t.Task.ID.String(), t.Task)

	w.updateTask(t.Task, inst)

	return result
}

func (w *Worker) updateTask(t domain.Task, inst ports.WorkerInstance) {
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		healthChan := make(chan *domain.ProcessHealthStatus)
		err := inst.StartMonitoring(ctx, 5, healthChan) // 5s
		if err != nil {
			log.Printf("Error abriendo canal de monitor para %s: %v", t.ID, err)
			return
		}

		for status := range healthChan {
			switch status.Status {
			case domain.RUNNING:
				// No action needed, the task is running
			case domain.HEALTHY:
				// No action needed, the task is healthy
			case domain.ERROR:
				log.Printf("Tarea %s: estado=%s => marcando como FAILED", t.ID, status.Status)
				t.State = domain.Failed
				_ = w.Db.Put(t.ID.String(), t)
				return
			case domain.FINISHED:
				log.Printf("Tarea %s: estado=%s => marcando como COMPLETED", t.ID, status.Status)
				t.State = domain.Completed
				_ = w.Db.Put(t.ID.String(), t)
				return
			case domain.STOPPED:
				log.Printf("Tarea %s: estado=%s => marcando como FAILED", t.ID, status.Status)
				t.State = domain.Stopped
				_ = w.Db.Put(t.ID.String(), t)
				return
			case domain.UNKNOWN:
				log.Printf("Tarea %s: estado=%s => marcando como FAILED", t.ID, status.Status)
				t.State = domain.Unknown
				_ = w.Db.Put(t.ID.String(), t)
				return
			default:
				log.Printf("Tarea %s: estado desconocido=%s => marcando como FAILED", t.ID, status.Status)
				t.State = domain.Failed
				_ = w.Db.Put(t.ID.String(), t)
				return
			}
		}
	}()
}

func (w *Worker) StopTask(t domain.Task) domain.TaskResult {
	inst, ok := w.workerInstances[t.Status.WorkerID.ID]
	if !ok {
		return domain.TaskResult{Error: fmt.Errorf("worker instance not found: %v", t.Status.WorkerID.ID)}
	}

	_, msg, err := inst.Stop()
	if err != nil {
		return domain.TaskResult{Error: fmt.Errorf("error stopping worker instance: %v, %S", err, msg)}
	}

	t.FinishTime = time.Now().UTC()
	t.State = domain.Completed
	_ = w.Db.Put(t.ID.String(), t)
	log.Printf("Stopped & removed container %v for task %v\n", t.Status.WorkerID, t.ID)

	return domain.TaskResult{Action: "stop", Result: "stopped"}
}
