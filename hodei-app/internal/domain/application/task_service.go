package usecases

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"fmt"
	"github.com/go-playground/validator"

	"time"
)

type TaskServiceImpl struct {
	repo      ports.Repository[*model.Task, model.AggregateID]
	validator *validator.Validate
}

func NewTaskService(repo ports.Repository[*model.Task, model.AggregateID]) ports.TaskService {
	if repo == nil {
		panic("repository cannot be nil")
	}

	validate := validator.New()
	// Registrar validaciones personalizadas para ParamType
	validate.RegisterValidation("paramtype", validateParamType)

	return &TaskServiceImpl{
		repo:      repo,
		validator: validate,
	}
}

func (s *TaskServiceImpl) CreateTask(ctx context.Context, task *model.Task) (*model.Task, error) {
	if err := s.validator.Struct(task); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	now := time.Now().UTC()
	if task.Metadata.CreatedAt.IsZero() {
		task.Metadata.CreatedAt = now
	}
	task.Metadata.UpdatedAt = now

	return s.repo.Save(ctx, task)
}

func (s *TaskServiceImpl) UpdateTask(ctx context.Context, id model.AggregateID, updates *model.Task) error {
	if err := s.validator.Var(id, "required"); err != nil {
		return fmt.Errorf("invalid id: %w", err)
	}

	if err := s.validator.Struct(updates); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find task: %w", err)
	}

	updates.ID = current.ID
	updates.Metadata.CreatedAt = current.Metadata.CreatedAt
	updates.Metadata.UpdatedAt = time.Now().UTC()

	return s.repo.Update(ctx, updates)
}

func (s *TaskServiceImpl) DeleteTask(ctx context.Context, id model.AggregateID) error {
	if err := s.validator.Var(id, "required"); err != nil {
		return fmt.Errorf("invalid id: %w", err)
	}

	exists, err := s.repo.Exists(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to check task existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("task not found with id: %s", id)
	}

	return s.repo.Delete(ctx, id)
}

func (s *TaskServiceImpl) GetTask(ctx context.Context, id model.AggregateID) (*model.Task, error) {
	if err := s.validator.Var(id, "required"); err != nil {
		return nil, fmt.Errorf("invalid id: %w", err)
	}

	return s.repo.FindByID(ctx, id)
}

func (s *TaskServiceImpl) ListTasks(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.Task], error) {
	if err := s.validator.Struct(criteria); err != nil {
		return ports.SearchResult[*model.Task]{}, fmt.Errorf("invalid criteria: %w", err)
	}

	if criteria.Page < 1 {
		criteria.Page = 1
	}
	if criteria.Size < 1 {
		criteria.Size = 10
	}
	if criteria.Size > 100 {
		criteria.Size = 100
	}

	if criteria.SortBy == "" {
		criteria.SortBy = "metadata.name"
	}

	return s.repo.FindByCriteria(ctx, criteria)
}

// validateParamType valida que el tipo de parámetro sea uno de los permitidos
func validateParamType(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	validTypes := map[string]bool{
		"string":      true,
		"integer":     true,
		"number":      true,
		"boolean":     true,
		"select":      true,
		"multiselect": true,
		"object":      true,
		"array":       true,
		"date":        true,
		"datetime":    true,
		"file":        true,
		"password":    true,
	}
	return validTypes[value]
}
