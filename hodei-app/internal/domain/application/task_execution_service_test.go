package usecases_test

import (
	"context"
	usecases "dev.rubentxu.hodei-devops/hodei-app/internal/domain/application"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var _ ports.TaskExecutionService = (*usecases.TaskExecutionServiceImpl)(nil)
var _ ports.Repository[*model.TaskExecution, model.AggregateID] = (*MockExecutionRepository)(nil)

type MockExecutionRepository struct {
	mock.Mock
}

func (m *MockExecutionRepository) Save(ctx context.Context, entity *model.TaskExecution) (*model.TaskExecution, error) {
	args := m.Called(ctx, entity)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.TaskExecution), args.Error(1)
}

func (m *MockExecutionRepository) Update(ctx context.Context, entity *model.TaskExecution) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockExecutionRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.TaskExecution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.TaskExecution), args.Error(1)
}

func (m *MockExecutionRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.TaskExecution], error) {
	args := m.Called(ctx, criteria)
	return args.Get(0).(ports.SearchResult[*model.TaskExecution]), args.Error(1)
}

func (m *MockExecutionRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockExecutionRepository) FindAll(ctx context.Context) ([]*model.TaskExecution, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.TaskExecution), args.Error(1)
}

func (m *MockExecutionRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockExecutionRepository) Delete(ctx context.Context, id model.AggregateID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockExecutionRepository) BatchSave(ctx context.Context, entities []*model.TaskExecution) ([]*model.TaskExecution, error) {
	args := m.Called(ctx, entities)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.TaskExecution), args.Error(1)
}

func (m *MockExecutionRepository) BatchUpdate(ctx context.Context, entities []*model.TaskExecution) error {
	args := m.Called(ctx, entities)
	return args.Error(0)
}

func (m *MockExecutionRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

type MockIDTEGenerator struct {
	mock.Mock
}

func (m *MockIDTEGenerator) NewID() model.AggregateID {
	args := m.Called()
	return args.Get(0).(model.AggregateID)
}

func createTestTaskExecution(id string) *model.TaskExecution {
	return &model.TaskExecution{
		ID: model.AggregateID(id),
		Metadata: model.Metadata{
			Name:        fmt.Sprintf("Task %s", id),
			Description: "Test task execution",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		Status: model.ExecutionStatus{
			State:     model.Pending,
			StartTime: time.Now(),
		},
		Task: model.Task{
			ID: model.AggregateID(fmt.Sprintf("task-%s", id)),
			Metadata: model.Metadata{
				Name: fmt.Sprintf("Task definition %s", id),
			},
		},
		WorkerDef: &model.WorkerDefinition{
			ID: model.AggregateID(fmt.Sprintf("worker-%s", id)),
		},
	}
}

func TestTaskExecutionService_CreateTaskExecution(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := &MockExecutionRepository{}
	mockIDGen := &MockIDTEGenerator{}
	service := usecases.NewTaskExecutionServiceImpl(mockRepo, mockIDGen)

	t.Run("Ejecución exitosa", func(t *testing.T) {
		execution := createTestTaskExecution("test-id")
		mockRepo.On("Save", ctx, execution).Return(execution, nil).Once()

		// Act
		result, err := service.CreateTaskExecution(ctx, execution)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, execution, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error al guardar", func(t *testing.T) {
		execution := createTestTaskExecution("error-id")
		mockRepo.On("Save", ctx, execution).Return(nil, errors.New("error de base de datos")).Once()

		// Act
		result, err := service.CreateTaskExecution(ctx, execution)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		mockRepo.AssertExpectations(t)
	})
}

func TestTaskExecutionService_GetTaskExecution(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := &MockExecutionRepository{}
	mockIDGen := &MockIDTEGenerator{}
	service := usecases.NewTaskExecutionServiceImpl(mockRepo, mockIDGen)

	t.Run("Obtención exitosa", func(t *testing.T) {
		id := model.AggregateID("test-id")
		expected := createTestTaskExecution(id.String())

		mockRepo.On("FindByID", ctx, id).Return(expected, nil).Once()

		// Act
		result, err := service.GetTaskExecution(ctx, id)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Ejecución no encontrada", func(t *testing.T) {
		id := model.AggregateID("non-existent-id")
		mockRepo.On("FindByID", ctx, id).Return(nil, ports.ErrNotFound).Once()

		// Act
		result, err := service.GetTaskExecution(ctx, id)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		mockRepo.AssertExpectations(t)
	})
}

func TestTaskExecutionService_ListTaskExecutions(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := &MockExecutionRepository{}
	mockIDGen := &MockIDTEGenerator{}
	service := usecases.NewTaskExecutionServiceImpl(mockRepo, mockIDGen)

	t.Run("Lista con resultados", func(t *testing.T) {
		criteria := ports.SearchCriteria{
			Page: 1,
			Size: 10,
		}

		expected := ports.SearchResult[*model.TaskExecution]{
			Content:       []*model.TaskExecution{createTestTaskExecution("1"), createTestTaskExecution("2")},
			TotalElements: 2,
			TotalPages:    1,
			Page:          1,
			Size:          10,
			HasNext:       false,
			HasPrevious:   false,
		}

		mockRepo.On("FindByCriteria", ctx, criteria).Return(expected, nil).Once()

		// Act
		result, err := service.ListTaskExecutions(ctx, criteria)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
		assert.Equal(t, 2, len(result.Content))
		mockRepo.AssertExpectations(t)
	})

	t.Run("Lista vacía", func(t *testing.T) {
		criteria := ports.SearchCriteria{
			Page: 1,
			Size: 10,
			Filters: map[string]interface{}{
				"state": model.Failed,
			},
		}

		expected := ports.SearchResult[*model.TaskExecution]{
			Content:       []*model.TaskExecution{},
			TotalElements: 0,
			TotalPages:    0,
			Page:          1,
			Size:          10,
			HasNext:       false,
			HasPrevious:   false,
		}

		mockRepo.On("FindByCriteria", ctx, criteria).Return(expected, nil).Once()

		// Act
		result, err := service.ListTaskExecutions(ctx, criteria)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 0, len(result.Content))
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error en repositorio", func(t *testing.T) {
		criteria := ports.SearchCriteria{
			Page: 1,
			Size: 10,
		}

		emptyResult := ports.SearchResult[*model.TaskExecution]{
			Content: []*model.TaskExecution{},
		}

		mockRepo.On("FindByCriteria", ctx, criteria).Return(emptyResult, errors.New("error de base de datos")).Once()

		// Act
		_, err := service.ListTaskExecutions(ctx, criteria)

		// Assert
		assert.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Filtros complejos", func(t *testing.T) {
		criteria := ports.SearchCriteria{
			Page: 1,
			Size: 10,
			Filters: map[string]interface{}{
				"state":        model.Running,
				"nameContains": "test",
			},
			SortBy:    "startTime",
			SortOrder: "DESC",
		}

		expected := ports.SearchResult[*model.TaskExecution]{
			Content:       []*model.TaskExecution{createTestTaskExecution("filtered-1")},
			TotalElements: 1,
		}

		mockRepo.On("FindByCriteria", ctx, criteria).Return(expected, nil).Once()

		// Act
		result, err := service.ListTaskExecutions(ctx, criteria)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 1, len(result.Content))
		mockRepo.AssertExpectations(t)
	})
}

func TestTaskExecutionService_UpdateTaskExecutionStatus(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := &MockExecutionRepository{}
	mockIDGen := &MockIDTEGenerator{}
	service := usecases.NewTaskExecutionServiceImpl(mockRepo, mockIDGen)

	t.Run("Actualización exitosa", func(t *testing.T) {
		id := model.AggregateID("test-id")
		execution := createTestTaskExecution(id.String())
		execution.Status = model.ExecutionStatus{State: model.Pending}

		newStatus := model.ExecutionStatus{State: model.Running}

		mockRepo.On("FindByID", ctx, id).Return(execution, nil).Once()
		mockRepo.On("Update", ctx, mock.MatchedBy(func(e *model.TaskExecution) bool {
			return e.ID == id && e.Status.State == model.Running
		})).Return(nil).Once()

		// Act
		err := service.UpdateTaskExecutionStatus(ctx, id, newStatus)

		// Assert
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Actualización a estado terminal", func(t *testing.T) {
		id := model.AggregateID("test-id")
		execution := createTestTaskExecution(id.String())
		execution.Status = model.ExecutionStatus{State: model.Running}

		// Variable para capturar el objeto actualizado
		var updatedExecution *model.TaskExecution

		newStatus := model.ExecutionStatus{
			State:   model.Completed,
			EndTime: time.Now(),
		}

		mockRepo.On("FindByID", ctx, id).Return(execution, nil).Once()
		mockRepo.On("Update", ctx, mock.AnythingOfType("*model.TaskExecution")).
			Run(func(args mock.Arguments) {
				// Capturar el objeto que se pasa a Update
				updatedExecution = args.Get(1).(*model.TaskExecution)
			}).
			Return(nil).Once()

		// Act
		err := service.UpdateTaskExecutionStatus(ctx, id, newStatus)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, updatedExecution, "La ejecución actualizada no debería ser nil")
		assert.Equal(t, id, updatedExecution.ID)
		assert.Equal(t, model.Completed, updatedExecution.Status.State)
		assert.False(t, updatedExecution.Status.EndTime.IsZero(), "EndTime debería estar establecido")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Ejecución no encontrada", func(t *testing.T) {
		id := model.AggregateID("non-existent-id")
		newStatus := model.ExecutionStatus{State: model.Running}

		mockRepo.On("FindByID", ctx, id).Return(nil, errors.New("not found")).Once()

		// Act
		err := service.UpdateTaskExecutionStatus(ctx, id, newStatus)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error al obtener la ejecución")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error al actualizar", func(t *testing.T) {
		id := model.AggregateID("error-id")
		execution := createTestTaskExecution(id.String())
		newStatus := model.ExecutionStatus{State: model.Running}

		mockRepo.On("FindByID", ctx, id).Return(execution, nil).Once()
		mockRepo.On("Update", ctx, mock.AnythingOfType("*model.TaskExecution")).Return(errors.New("error de actualización")).Once()

		// Act
		err := service.UpdateTaskExecutionStatus(ctx, id, newStatus)

		// Assert
		assert.Error(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestTaskExecutionService_CancelTaskExecution(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := &MockExecutionRepository{}
	mockIDGen := &MockIDTEGenerator{}
	service := usecases.NewTaskExecutionServiceImpl(mockRepo, mockIDGen)

	t.Run("Cancelación exitosa", func(t *testing.T) {
		id := model.AggregateID("test-id")
		execution := createTestTaskExecution(id.String())
		execution.Status = model.ExecutionStatus{State: model.Running}

		mockRepo.On("FindByID", ctx, id).Return(execution, nil).Once()
		mockRepo.On("Update", ctx, mock.MatchedBy(func(e *model.TaskExecution) bool {
			return e.ID == id &&
				e.Status.State == model.Stopped &&
				!e.Status.EndTime.IsZero() &&
				e.Status.Message == "Cancelled by user"
		})).Return(nil).Once()

		// Act
		err := service.CancelTaskExecution(ctx, id)

		// Assert
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Cancelar una tarea ya completada", func(t *testing.T) {
		id := model.AggregateID("completed-id")
		execution := createTestTaskExecution(id.String())
		execution.Status = model.ExecutionStatus{
			State:   model.Done,
			EndTime: time.Now().UTC(),
		}

		mockRepo.On("FindByID", ctx, id).Return(execution, nil).Once()
		// No debemos esperar una llamada a Update ya que el servicio debe retornar un error

		// Act
		err := service.CancelTaskExecution(ctx, id)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no se puede cancelar una ejecución que ya está en estado terminal")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Cancelar una tarea fallida", func(t *testing.T) {
		id := model.AggregateID("failed-id")
		execution := createTestTaskExecution(id.String())
		execution.Status = model.ExecutionStatus{
			State:   model.Failed,
			EndTime: time.Now().Add(-1 * time.Hour),
		}

		mockRepo.On("FindByID", ctx, id).Return(execution, nil).Once()

		// Act
		err := service.CancelTaskExecution(ctx, id)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no se puede cancelar una ejecución que ya está en estado terminal")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Ejecución no encontrada", func(t *testing.T) {
		id := model.AggregateID("non-existent-id")
		mockRepo.On("FindByID", ctx, id).Return(nil, errors.New("not found")).Once()

		// Act
		err := service.CancelTaskExecution(ctx, id)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error al obtener la ejecución")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error al actualizar", func(t *testing.T) {
		id := model.AggregateID("update-error-id")
		execution := createTestTaskExecution(id.String())
		execution.Status = model.ExecutionStatus{State: model.Running}

		mockRepo.On("FindByID", ctx, id).Return(execution, nil).Once()
		mockRepo.On("Update", ctx, mock.AnythingOfType("*model.TaskExecution")).Return(errors.New("error de actualización")).Once()

		// Act
		err := service.CancelTaskExecution(ctx, id)

		// Assert
		assert.Error(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestTaskExecutionService_GetTaskExecutionMetrics(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := &MockExecutionRepository{}
	mockIDGen := &MockIDTEGenerator{}
	service := usecases.NewTaskExecutionServiceImpl(mockRepo, mockIDGen)

	t.Run("Obtener métricas exitosamente", func(t *testing.T) {
		mockRepo.On("Count", ctx).Return(int64(10), nil).Once()

		// Configurar respuestas para todos los estados posibles
		for _, state := range model.AllTaskStates() {
			stateValue := state // Importante: crear una variable local para cada estado
			mockRepo.On("FindByCriteria", ctx, mock.MatchedBy(func(c ports.SearchCriteria) bool {
				s, ok := c.Filters["state"]
				return ok && s == stateValue
			})).Return(ports.SearchResult[*model.TaskExecution]{
				Content:       []*model.TaskExecution{createTestTaskExecution(fmt.Sprintf("state-%d-1", state))},
				TotalElements: 1,
			}, nil).Once()
		}

		// Para Completed (para calcular tiempo promedio)
		mockRepo.On("FindByCriteria", ctx, mock.MatchedBy(func(c ports.SearchCriteria) bool {
			s, ok := c.Filters["state"]
			return ok && s == model.Completed
		})).Return(ports.SearchResult[*model.TaskExecution]{
			Content: []*model.TaskExecution{
				func() *model.TaskExecution {
					e := createTestTaskExecution("completed-1")
					e.Status.StartTime = time.Now().Add(-20 * time.Minute)
					e.Status.EndTime = time.Now().Add(-10 * time.Minute)
					return e
				}(),
				func() *model.TaskExecution {
					e := createTestTaskExecution("completed-2")
					e.Status.StartTime = time.Now().Add(-30 * time.Minute)
					e.Status.EndTime = time.Now().Add(-15 * time.Minute)
					return e
				}(),
			},
			TotalElements: 2,
		}, nil).Once()

		// Act
		metrics, err := service.GetTaskExecutionMetrics(ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, int64(10), metrics.TotalExecutions)
		assert.Equal(t, int64(1), metrics.ExecutionsByState[model.Pending])
		assert.Equal(t, int64(1), metrics.ExecutionsByState[model.Running])
		assert.Equal(t, int64(1), metrics.ExecutionsByState[model.Completed])
		assert.Greater(t, metrics.AverageExecutionTime, float64(0))
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error al obtener conteo", func(t *testing.T) {
		mockRepo.On("Count", ctx).Return(int64(0), errors.New("error de base de datos")).Once()

		// Act
		metrics, err := service.GetTaskExecutionMetrics(ctx)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error al obtener el total de ejecuciones")
		assert.Equal(t, int64(0), metrics.TotalExecutions)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error al obtener métricas por estado", func(t *testing.T) {
		mockRepo.On("Count", ctx).Return(int64(10), nil).Once()

		mockRepo.On("FindByCriteria", ctx, mock.MatchedBy(func(c ports.SearchCriteria) bool {
			return c.Filters["state"] == model.Pending
		})).Return(ports.SearchResult[*model.TaskExecution]{}, errors.New("error de consulta")).Once()

		// Act
		_, err := service.GetTaskExecutionMetrics(ctx)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error al obtener métricas por estado")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error al obtener tareas completadas", func(t *testing.T) {
		mockRepo.On("Count", ctx).Return(int64(10), nil).Once()

		// Configurar para todos los estados excepto completadas
		for _, state := range []model.TaskState{
			model.Pending, model.Running, model.Failed,
			model.Stopped, model.Scheduled, model.Skipped,
		} {
			mockRepo.On("FindByCriteria", ctx, mock.MatchedBy(func(c ports.SearchCriteria) bool {
				return c.Filters["state"] == state
			})).Return(ports.SearchResult[*model.TaskExecution]{}, nil).Once()
		}

		// Error solo en el estado completado
		mockRepo.On("FindByCriteria", ctx, mock.MatchedBy(func(c ports.SearchCriteria) bool {
			return c.Filters["state"] == model.Completed
		})).Return(ports.SearchResult[*model.TaskExecution]{}, errors.New("error al obtener tareas completadas")).Once()

		// Act
		_, err := service.GetTaskExecutionMetrics(ctx)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error al obtener tareas completadas")
		//mockRepo.AssertExpectations(t)
	})

	t.Run("Sin tareas completadas", func(t *testing.T) {
		// Arrange con nuevo mock
		ctx := context.Background()
		mockRepo := &MockExecutionRepository{}
		mockIDGen := &MockIDTEGenerator{}
		service := usecases.NewTaskExecutionServiceImpl(mockRepo, mockIDGen)

		mockRepo.On("Count", ctx).Return(int64(5), nil).Once()

		// Configurar para todos los estados (incluido Completed) con respuesta vacía
		for _, state := range model.AllTaskStates() {
			stateValue := state
			// Configuramos dos llamadas para Completed
			if state == model.Completed {
				// Primera llamada para contar
				mockRepo.On("FindByCriteria", ctx, mock.MatchedBy(func(c ports.SearchCriteria) bool {
					s, ok := c.Filters["state"]
					return ok && s == stateValue
				})).Return(ports.SearchResult[*model.TaskExecution]{
					Content:       []*model.TaskExecution{},
					TotalElements: 0,
				}, nil).Once()

				// Segunda llamada para calcular tiempo
				mockRepo.On("FindByCriteria", ctx, mock.MatchedBy(func(c ports.SearchCriteria) bool {
					s, ok := c.Filters["state"]
					return ok && s == stateValue
				})).Return(ports.SearchResult[*model.TaskExecution]{
					Content:       []*model.TaskExecution{},
					TotalElements: 0,
				}, nil).Once()
			} else {
				// Una sola llamada para otros estados
				mockRepo.On("FindByCriteria", ctx, mock.MatchedBy(func(c ports.SearchCriteria) bool {
					s, ok := c.Filters["state"]
					return ok && s == stateValue
				})).Return(ports.SearchResult[*model.TaskExecution]{
					Content:       []*model.TaskExecution{},
					TotalElements: 0,
				}, nil).Once()
			}
		}

		// Act
		metrics, err := service.GetTaskExecutionMetrics(ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, int64(5), metrics.TotalExecutions)
		assert.Equal(t, float64(0), metrics.AverageExecutionTime)
		mockRepo.AssertExpectations(t)
	})
}

// TestTaskExecutionService_Edge_Cases prueba casos límite adicionales
func TestTaskExecutionService_Edge_Cases(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := &MockExecutionRepository{}
	mockIDGen := &MockIDTEGenerator{}
	service := usecases.NewTaskExecutionServiceImpl(mockRepo, mockIDGen)

	t.Run("Actualizar estado en tarea con error transitorio", func(t *testing.T) {
		// Simula un error transitorio donde la primera llamada falla pero la segunda tiene éxito
		id := model.AggregateID("test-retry")
		execution := createTestTaskExecution(id.String())
		execution.Status = model.ExecutionStatus{State: model.Pending}
		newStatus := model.ExecutionStatus{State: model.Running}

		// Primera llamada falla
		mockRepo.On("FindByID", ctx, id).Return(execution, nil).Once()
		mockRepo.On("Update", ctx, mock.AnythingOfType("*model.TaskExecution")).Return(errors.New("error transitorio")).Once()

		// Act - Primera intento
		err1 := service.UpdateTaskExecutionStatus(ctx, id, newStatus)

		// Assert primer intento
		assert.Error(t, err1)

		// Reset y configurar para segundo intento
		mockRepo = &MockExecutionRepository{}
		service = usecases.NewTaskExecutionServiceImpl(mockRepo, mockIDGen)
		mockRepo.On("FindByID", ctx, id).Return(execution, nil).Once()
		mockRepo.On("Update", ctx, mock.AnythingOfType("*model.TaskExecution")).Return(nil).Once()

		// Act - Segundo intento
		err2 := service.UpdateTaskExecutionStatus(ctx, id, newStatus)

		// Assert segundo intento
		assert.NoError(t, err2)
		mockRepo.AssertExpectations(t)
	})
}
