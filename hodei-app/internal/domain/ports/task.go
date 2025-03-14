package ports

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
)

type TaskService interface {
	CreateTask(ctx context.Context, task *model.Task) (*model.Task, error)
	UpdateTask(ctx context.Context, id model.AggregateID, updates *model.Task) error
	DeleteTask(ctx context.Context, id model.AggregateID) error
	GetTask(ctx context.Context, id model.AggregateID) (*model.Task, error)
	ListTasks(ctx context.Context, criteria SearchCriteria) (SearchResult[*model.Task], error)
}

type TaskExecutionService interface {
	CreateTaskExecution(ctx context.Context, execution *model.TaskExecution) (*model.TaskExecution, error)
	GetTaskExecution(ctx context.Context, id model.AggregateID) (*model.TaskExecution, error)
	ListTaskExecutions(ctx context.Context, criteria SearchCriteria) (SearchResult[*model.TaskExecution], error)
	UpdateTaskExecutionStatus(ctx context.Context, id model.AggregateID, status model.ExecutionStatus) error
	CancelTaskExecution(ctx context.Context, id model.AggregateID) error
	GetTaskExecutionMetrics(ctx context.Context) (TaskExecutionMetrics, error)
}

type TaskExecutionMetrics struct {
	TotalExecutions      int64                     `json:"total_executions"`
	ExecutionsByState    map[model.TaskState]int64 `json:"executions_by_state"`
	AverageExecutionTime float64                   `json:"average_execution_time"`
}
