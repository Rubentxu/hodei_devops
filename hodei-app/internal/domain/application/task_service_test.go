package usecases_test

import (
	"context"
	usecases "dev.rubentxu.hodei-devops/hodei-app/internal/domain/application"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

type MockTaskRepository struct {
	mock.Mock
	ports.Repository[*model.Task, model.AggregateID]
}

func (m *MockTaskRepository) Save(ctx context.Context, entity *model.Task) (*model.Task, error) {
	args := m.Called(ctx, entity)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Task), args.Error(1)
}

func (m *MockTaskRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Task), args.Error(1)
}

func (m *MockTaskRepository) Update(ctx context.Context, entity *model.Task) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockTaskRepository) Delete(ctx context.Context, id model.AggregateID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTaskRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockTaskRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.Task], error) {
	args := m.Called(ctx, criteria)
	return args.Get(0).(ports.SearchResult[*model.Task]), args.Error(1)
}

func createTestTask() *model.Task {
	return &model.Task{
		ID: "test-id",
		Metadata: model.Metadata{
			Name:        "Test Task",
			Description: "Test Description",
			Labels:      []string{"test"},
			Annotations: map[string]string{"env": "test"},
		},
		Spec: model.TaskSpec{
			WorkerDefinitionName: "worker-1",
			Command:              []string{"echo", "hello"},
			Params: []model.ParamDefinition{
				{
					Key:         "param1",
					Type:        model.ParamTypeString,
					Label:       "Parameter 1",
					Description: "Test parameter",
					Required:    true,
				},
			},
		},
	}
}

func TestTaskService(t *testing.T) {
	ctx := context.Background()
	mockRepo := &MockTaskRepository{}
	service := usecases.NewTaskService(mockRepo)

	t.Run("CreateTask", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			task := createTestTask()
			mockRepo.On("Save", ctx, mock.AnythingOfType("*model.Task")).Return(task, nil)

			created, err := service.CreateTask(ctx, task)
			require.NoError(t, err)
			assert.Equal(t, task.ID, created.ID)
			assert.False(t, created.Metadata.CreatedAt.IsZero())
			assert.False(t, created.Metadata.UpdatedAt.IsZero())
		})

		t.Run("Validation Error", func(t *testing.T) {
			task := createTestTask()
			task.Spec.Command = []string{} // Viola la validación min=1

			_, err := service.CreateTask(ctx, task)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "validation failed")
		})
	})

	t.Run("UpdateTask", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			task := createTestTask()
			mockRepo.On("FindByID", ctx, task.ID).Return(task, nil)
			mockRepo.On("Update", ctx, mock.AnythingOfType("*model.Task")).Return(nil)

			err := service.UpdateTask(ctx, task.ID, task)
			require.NoError(t, err)
		})

		t.Run("Not Found", func(t *testing.T) {
			task := createTestTask()
			task.ID = "" // ID vacío provocará error de validación
			mockRepo.On("FindByID", ctx, task.ID).Return(nil, nil)

			err := service.UpdateTask(ctx, task.ID, task)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid id: Key")
		})
	})

	t.Run("DeleteTask", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			task := createTestTask()
			mockRepo.On("Exists", ctx, task.ID).Return(true, nil)
			mockRepo.On("Delete", ctx, task.ID).Return(nil)

			err := service.DeleteTask(ctx, task.ID)
			require.NoError(t, err)
		})

		t.Run("Not Found", func(t *testing.T) {
			id := model.AggregateID("") // ID vacío provocará error de validación
			mockRepo.On("Exists", ctx, id).Return(false, nil)

			err := service.DeleteTask(ctx, id)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid id: Key")
		})
	})

	t.Run("ListTasks", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			mockRepo.ExpectedCalls = nil // Limpiar mocks anteriores

			criteria := ports.SearchCriteria{
				Page:      1,
				Size:      10,
				SortBy:    "name",
				SortOrder: "ASC",
			}

			expectedResult := ports.SearchResult[*model.Task]{
				Content:       []*model.Task{createTestTask()},
				TotalElements: 1,
				Page:          1,
				Size:          10,
			}

			mockRepo.On("FindByCriteria", ctx, criteria).Return(expectedResult, nil)

			result, err := service.ListTasks(ctx, criteria)
			require.NoError(t, err)
			assert.Equal(t, expectedResult.TotalElements, result.TotalElements)
			assert.Equal(t, len(expectedResult.Content), len(result.Content))
			mockRepo.AssertExpectations(t)
		})

		t.Run("Invalid Criteria", func(t *testing.T) {
			invalidCriteria := []struct {
				name     string
				criteria ports.SearchCriteria
			}{
				{
					name: "Página negativa",
					criteria: ports.SearchCriteria{
						Page:      -1,
						Size:      10,
						SortBy:    "name",
						SortOrder: "ASC",
					},
				},
				{
					name: "Tamaño negativo",
					criteria: ports.SearchCriteria{
						Page:      1,
						Size:      -1,
						SortBy:    "name",
						SortOrder: "ASC",
					},
				},
				{
					name: "Orden inválido",
					criteria: ports.SearchCriteria{
						Page:      1,
						Size:      10,
						SortBy:    "name",
						SortOrder: "INVALID",
					},
				},
			}

			for _, tc := range invalidCriteria {
				t.Run(tc.name, func(t *testing.T) {
					mockRepo.ExpectedCalls = nil // Limpiar mocks para cada subtest

					// No configuramos expectativas del mock porque esperamos que falle en la validación
					result, err := service.ListTasks(ctx, tc.criteria)
					require.Error(t, err)
					assert.Empty(t, result.Content)
					assert.Contains(t, err.Error(), "invalid criteria")
				})
			}
		})
	})

	t.Run("GetTask", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			task := createTestTask()
			mockRepo.On("FindByID", ctx, task.ID).Return(task, nil)

			found, err := service.GetTask(ctx, task.ID)
			require.NoError(t, err)
			assert.Equal(t, task.ID, found.ID)
		})

		t.Run("Not Found", func(t *testing.T) {
			id := model.AggregateID("non-existent")
			mockRepo.On("FindByID", ctx, id).Return(nil, ports.ErrNotFound)

			_, err := service.GetTask(ctx, id)
			require.Error(t, err)
		})
	})
}
