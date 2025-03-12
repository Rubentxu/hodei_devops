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
