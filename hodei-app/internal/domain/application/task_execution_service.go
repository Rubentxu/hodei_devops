// application/task_execution_service.go
package usecases

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"fmt"
	"time"
)

var _ ports.TaskExecutionService = (*TaskExecutionServiceImpl)(nil)

type TaskExecutionServiceImpl struct {
	repository  ports.Repository[*model.TaskExecution, model.AggregateID]
	idGenerator ports.IDGenerator
}

func NewTaskExecutionServiceImpl(
	repository ports.Repository[*model.TaskExecution, model.AggregateID],
	idGenerator ports.IDGenerator,
) ports.TaskExecutionService {
	return &TaskExecutionServiceImpl{
		repository:  repository,
		idGenerator: idGenerator,
	}
}

func (s *TaskExecutionServiceImpl) CreateTaskExecution(ctx context.Context, execution *model.TaskExecution) (*model.TaskExecution, error) {
	return s.repository.Save(ctx, execution)
}

func (s *TaskExecutionServiceImpl) GetTaskExecution(ctx context.Context, id model.AggregateID) (*model.TaskExecution, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *TaskExecutionServiceImpl) ListTaskExecutions(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.TaskExecution], error) {
	return s.repository.FindByCriteria(ctx, criteria)
}

func (s *TaskExecutionServiceImpl) UpdateTaskExecutionStatus(ctx context.Context, id model.AggregateID, status model.ExecutionStatus) error {
	execution, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("error al obtener la ejecución: %w", err)
	}

	execution.Status = status
	if status.State.IsTerminal() {
		execution.Status.EndTime = time.Now().UTC()
	}

	return s.repository.Update(ctx, execution)
}

func (s *TaskExecutionServiceImpl) CancelTaskExecution(ctx context.Context, id model.AggregateID) error {
	execution, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("error al obtener la ejecución: %w", err)
	}

	if execution.Status.State.IsTerminal() {
		return fmt.Errorf("no se puede cancelar una ejecución que ya está en estado terminal")
	}

	execution.Status.State = model.Stopped
	execution.Status.EndTime = time.Now().UTC()
	execution.Status.Message = "Cancelled by user"

	return s.repository.Update(ctx, execution)
}

func (s *TaskExecutionServiceImpl) GetTaskExecutionMetrics(ctx context.Context) (ports.TaskExecutionMetrics, error) {
	metrics := ports.TaskExecutionMetrics{
		ExecutionsByState: make(map[model.TaskState]int64),
	}

	total, err := s.repository.Count(ctx)
	if err != nil {
		return metrics, fmt.Errorf("error al obtener el total de ejecuciones: %w", err)
	}
	metrics.TotalExecutions = total

	// Obtener conteos por estado usando búsqueda por criterios
	for _, state := range model.AllTaskStates() {
		result, err := s.repository.FindByCriteria(ctx, ports.SearchCriteria{
			Filters: map[string]interface{}{"state": state},
		})
		if err != nil {
			return metrics, fmt.Errorf("error al obtener métricas por estado: %w", err)
		}
		metrics.ExecutionsByState[state] = int64(result.TotalElements)
	}

	// Calcular tiempo promedio de ejecución para tareas completadas
	completedTasks, err := s.repository.FindByCriteria(ctx, ports.SearchCriteria{
		Filters: map[string]interface{}{"state": model.Completed},
	})
	if err != nil {
		return metrics, fmt.Errorf("error al obtener tareas completadas: %w", err)
	}

	var totalDuration time.Duration
	for _, task := range completedTasks.Content {
		duration := task.Status.EndTime.Sub(task.Status.StartTime)
		totalDuration += duration
	}

	if len(completedTasks.Content) > 0 {
		metrics.AverageExecutionTime = totalDuration.Seconds() / float64(len(completedTasks.Content))
	}

	return metrics, nil
}
