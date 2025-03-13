package usecases_test

import (
	"context"
	usecases "dev.rubentxu.hodei-devops/hodei-app/internal/domain/application"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// Mock para el repositorio
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.WorkerDefinition, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.WorkerDefinition), args.Error(1)
}

func (m *MockRepository) FindAll(ctx context.Context) ([]*model.WorkerDefinition, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*model.WorkerDefinition), args.Error(1)
}

func (m *MockRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.WorkerDefinition], error) {
	args := m.Called(ctx, criteria)
	return args.Get(0).(ports.SearchResult[*model.WorkerDefinition]), args.Error(1)
}

func (m *MockRepository) Save(ctx context.Context, entity *model.WorkerDefinition) (*model.WorkerDefinition, error) {
	args := m.Called(ctx, entity)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.WorkerDefinition), args.Error(1)
}

func (m *MockRepository) Update(ctx context.Context, entity *model.WorkerDefinition) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockRepository) Delete(ctx context.Context, id model.AggregateID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) BatchSave(ctx context.Context, entities []*model.WorkerDefinition) ([]*model.WorkerDefinition, error) {
	args := m.Called(ctx, entities)
	return args.Get(0).([]*model.WorkerDefinition), args.Error(1)
}

func (m *MockRepository) BatchUpdate(ctx context.Context, entities []*model.WorkerDefinition) error {
	args := m.Called(ctx, entities)
	return args.Error(0)
}

func (m *MockRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

// Mock para el generador de IDs
type MockIDGenerator struct {
	mock.Mock
}

func (m *MockIDGenerator) NewID() model.AggregateID {
	args := m.Called()
	return args.Get(0).(model.AggregateID)
}

func createTestWorkerDef(name string, status model.HealthStatus) *model.WorkerDefinition {
	now := time.Now().UTC()
	return &model.WorkerDefinition{
		Metadata: model.Metadata{
			Name:        name, // Usar el nombre proporcionado
			Description: "Descripción de prueba",
			Labels:      []string{"test", "worker"},
			Annotations: map[string]string{"env": "test"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Spec: model.WorkerSpec{
			Type:       model.DockerInstance,
			Image:      "test-image:latest",
			Env:        map[string]string{"TEST": "value"},
			WorkingDir: "/app",
			Resources: model.ResourceRequirements{
				CPU:    1.0,
				Memory: "1Gi",
			},
			Labels: map[string]string{"app": "test"},
		},
		Status: model.WorkerStatus{
			Status: status,
		},
	}
}

func TestWorkerDefinitionService_CrearWorkerDefinition(t *testing.T) {
	// Preparar
	mockRepo := new(MockRepository)
	mockIDGen := new(MockIDGenerator)

	mockIDGen.On("NewID").Return(model.AggregateID("new-id"))

	// Configurar servicio
	service := usecases.NewWorkerDefinitionService(mockRepo, mockIDGen)
	ctx := context.Background()

	t.Run("Crear worker válido", func(t *testing.T) {
		// Preparar
		workerDef := createTestWorkerDef("Worker1", model.UNKNOWN)
		workerDef.ID = "" // Sin ID para que se genere uno nuevo

		expectedWorker := *workerDef
		expectedWorker.ID = "new-id"
		expectedWorker.Status.Status = model.PENDING

		mockRepo.On("Save", ctx, mock.AnythingOfType("*model.WorkerDefinition")).
			Return(&expectedWorker, nil).Once()

		// Ejecutar
		result, err := service.CreateWorkerDefinition(ctx, workerDef)

		// Verificar
		require.NoError(t, err)
		assert.Equal(t, model.AggregateID("new-id"), result.ID)
		assert.Equal(t, model.PENDING, result.Status.Status)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error al crear worker inválido", func(t *testing.T) {
		// Preparar
		ctx := context.Background()
		invalidWorker := createTestWorkerDef("", model.PENDING)
		invalidWorker.Metadata.Name = "" // Asegurarse de que el nombre esté vacío

		// No configuramos Mock.On("Save") porque esperamos que falle en la validación

		// Ejecutar
		_, err := service.CreateWorkerDefinition(ctx, invalidWorker)

		// Verificar
		assert.Error(t, err)
		assert.True(t, errors.Is(err, usecases.ErrInvalidWorker))
		mockRepo.AssertNotCalled(t, "Save") // Verificar que Save nunca fue llamado
	})

	t.Run("Error en repositorio", func(t *testing.T) {
		// Preparar
		workerDef := createTestWorkerDef("Worker3", model.PENDING)
		repoError := errors.New("error de base de datos")

		mockRepo.On("Save", ctx, mock.AnythingOfType("*model.WorkerDefinition")).
			Return(nil, repoError).Once()

		// Ejecutar
		_, err := service.CreateWorkerDefinition(ctx, workerDef)

		// Verificar
		require.Error(t, err)
		assert.Equal(t, repoError, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestWorkerDefinitionService_ObtenerWorkerDefinition(t *testing.T) {
	// Preparar
	mockRepo := new(MockRepository)
	mockIDGen := new(MockIDGenerator)
	service := usecases.NewWorkerDefinitionService(mockRepo, mockIDGen)
	ctx := context.Background()

	t.Run("Obtener worker existente", func(t *testing.T) {
		// Preparar
		id := model.AggregateID("existing-id")
		expectedWorker := createTestWorkerDef("Worker1", model.HEALTHY)
		expectedWorker.ID = id

		mockRepo.On("FindByID", ctx, id).Return(expectedWorker, nil).Once()

		// Ejecutar
		result, err := service.GetWorkerDefinition(ctx, id)

		// Verificar
		require.NoError(t, err)
		assert.Equal(t, expectedWorker.ID, result.ID)
		assert.Equal(t, expectedWorker.Metadata.Name, result.Metadata.Name)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Worker no encontrado", func(t *testing.T) {
		// Preparar
		id := model.AggregateID("non-existing")
		mockRepo.On("FindByID", ctx, id).Return(nil, errors.New("no encontrado")).Once()

		// Ejecutar
		_, err := service.GetWorkerDefinition(ctx, id)

		// Verificar
		require.Error(t, err)
		assert.Equal(t, usecases.ErrWorkerNotFound, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestWorkerDefinitionService_ActualizarWorkerDefinition(t *testing.T) {
	// Preparar
	mockRepo := new(MockRepository)
	mockIDGen := new(MockIDGenerator)
	service := usecases.NewWorkerDefinitionService(mockRepo, mockIDGen)
	ctx := context.Background()

	t.Run("Actualizar worker existente", func(t *testing.T) {
		// Preparar
		id := model.AggregateID("existing-id")
		existingWorker := createTestWorkerDef("WorkerOriginal", model.HEALTHY)
		existingWorker.ID = id

		updates := createTestWorkerDef("WorkerActualizado", model.HEALTHY)
		updates.ID = id

		mockRepo.On("FindByID", ctx, id).Return(existingWorker, nil).Once()
		mockRepo.On("Update", ctx, mock.AnythingOfType("*model.WorkerDefinition")).Return(nil).Once()

		// Ejecutar
		err := service.UpdateWorkerDefinition(ctx, updates)

		// Verificar
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Worker no encontrado", func(t *testing.T) {
		// Preparar
		id := model.AggregateID("non-existing")
		updates := createTestWorkerDef("Worker", model.HEALTHY)
		updates.ID = id

		mockRepo.On("FindByID", ctx, id).Return(nil, errors.New("no encontrado")).Once()

		// Ejecutar
		err := service.UpdateWorkerDefinition(ctx, updates)

		// Verificar
		require.Error(t, err)
		assert.Equal(t, usecases.ErrWorkerNotFound, err)
		mockRepo.AssertNotCalled(t, "Update")
	})
}

func TestWorkerDefinitionService_EliminarWorkerDefinition(t *testing.T) {
	// Preparar
	mockRepo := new(MockRepository)
	mockIDGen := new(MockIDGenerator)
	service := usecases.NewWorkerDefinitionService(mockRepo, mockIDGen)
	ctx := context.Background()

	t.Run("Eliminar worker existente", func(t *testing.T) {
		// Preparar
		id := model.AggregateID("existing-id")

		mockRepo.On("Exists", ctx, id).Return(true, nil).Once()
		mockRepo.On("Delete", ctx, id).Return(nil).Once()

		// Ejecutar
		err := service.DeleteWorkerDefinition(ctx, id)

		// Verificar
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Worker no encontrado", func(t *testing.T) {
		// Preparar
		id := model.AggregateID("non-existing")

		mockRepo.On("Exists", ctx, id).Return(false, nil).Once()

		// Ejecutar
		err := service.DeleteWorkerDefinition(ctx, id)

		// Verificar
		require.Error(t, err)
		assert.Equal(t, usecases.ErrWorkerNotFound, err)
		mockRepo.AssertNotCalled(t, "Delete")
	})
}

func TestWorkerDefinitionService_ListarWorkerDefinitions(t *testing.T) {
	// Preparar
	mockRepo := new(MockRepository)
	mockIDGen := new(MockIDGenerator)
	service := usecases.NewWorkerDefinitionService(mockRepo, mockIDGen)
	ctx := context.Background()

	t.Run("Listar workers con criterios", func(t *testing.T) {
		// Preparar
		criteria := ports.SearchCriteria{
			Filters:   map[string]interface{}{"type": "docker"},
			Page:      1,
			Size:      10,
			SortBy:    "name",
			SortOrder: "ASC",
		}

		workers := []*model.WorkerDefinition{
			createTestWorkerDef("Worker1", model.HEALTHY),
			createTestWorkerDef("Worker2", model.PENDING),
		}

		expectedResult := ports.SearchResult[*model.WorkerDefinition]{
			Content:       workers,
			TotalElements: 2,
			TotalPages:    1,
			Page:          1,
			Size:          10,
		}

		mockRepo.On("FindByCriteria", ctx, criteria).Return(expectedResult, nil).Once()

		// Ejecutar
		result, err := service.FindWorkerDefinitions(ctx, criteria)

		// Verificar
		require.NoError(t, err)
		assert.Equal(t, expectedResult.TotalElements, result.TotalElements)
		assert.Equal(t, len(expectedResult.Content), len(result.Content))
		mockRepo.AssertExpectations(t)
	})
}

func TestWorkerDefinitionService_ActualizarEstadoWorker(t *testing.T) {
	// Preparar
	mockRepo := new(MockRepository)
	mockIDGen := new(MockIDGenerator)
	service := usecases.NewWorkerDefinitionService(mockRepo, mockIDGen)
	ctx := context.Background()

	t.Run("Actualizar estado correctamente", func(t *testing.T) {
		// Preparar
		id := model.AggregateID("worker-id")
		existingWorker := createTestWorkerDef("Worker", model.PENDING)

		mockRepo.On("FindByID", ctx, id).Return(existingWorker, nil).Once()
		mockRepo.On("Update", ctx, mock.AnythingOfType("*model.WorkerDefinition")).Run(func(args mock.Arguments) {
			updatedWorker := args.Get(1).(*model.WorkerDefinition)
			assert.Equal(t, model.HEALTHY, updatedWorker.Status.Status)
		}).Return(nil).Once()

		// Ejecutar
		err := service.UpdateWorkerStatus(ctx, id, model.HEALTHY)

		// Verificar
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Worker no encontrado", func(t *testing.T) {
		// Preparar
		id := model.AggregateID("non-existing")

		mockRepo.On("FindByID", ctx, id).Return(nil, errors.New("no encontrado")).Once()

		// Ejecutar
		err := service.UpdateWorkerStatus(ctx, id, model.HEALTHY)

		// Verificar
		require.Error(t, err)
		assert.Equal(t, usecases.ErrWorkerNotFound, err)
		mockRepo.AssertNotCalled(t, "Update")
	})
}

func TestWorkerDefinitionService_AsociarTemplate(t *testing.T) {
	// Preparar
	mockRepo := new(MockRepository)
	mockIDGen := new(MockIDGenerator)
	service := usecases.NewWorkerDefinitionService(mockRepo, mockIDGen)
	ctx := context.Background()

	t.Run("Asociar template correctamente", func(t *testing.T) {
		// Preparar
		workerID := model.AggregateID("worker-id")
		templateID := "template-123"
		existingWorker := createTestWorkerDef("Worker", model.HEALTHY)

		mockRepo.On("FindByID", ctx, workerID).Return(existingWorker, nil).Once()
		mockRepo.On("Update", ctx, mock.AnythingOfType("*model.WorkerDefinition")).Run(func(args mock.Arguments) {
			updatedWorker := args.Get(1).(*model.WorkerDefinition)
			assert.Equal(t, templateID, updatedWorker.Spec.TemplateID)
		}).Return(nil).Once()

		// Ejecutar
		err := service.AssignTemplate(ctx, workerID, templateID)

		// Verificar
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestWorkerDefinitionService_CrearWorkerDefinitionsEnLote(t *testing.T) {
	// Preparar
	mockRepo := new(MockRepository)
	mockIDGen := new(MockIDGenerator)
	mockIDGen.On("NewID").Return(model.AggregateID("batch-id-1"))
	mockIDGen.On("NewID").Return(model.AggregateID("batch-id-2"))

	service := usecases.NewWorkerDefinitionService(mockRepo, mockIDGen)
	ctx := context.Background()

	t.Run("Crear lote correctamente", func(t *testing.T) {
		// Preparar
		workers := []*model.WorkerDefinition{
			createTestWorkerDef("Worker1", model.PENDING),
			createTestWorkerDef("Worker2", model.PENDING),
		}
		workers[0].ID = "" // Sin ID para probar la generación
		workers[1].ID = ""

		expectedWorkers := []*model.WorkerDefinition{
			createTestWorkerDef("Worker1", model.PENDING),
			createTestWorkerDef("Worker2", model.PENDING),
		}
		expectedWorkers[0].ID = "batch-id-1"
		expectedWorkers[1].ID = "batch-id-2"

		mockRepo.On("BatchSave", ctx, mock.AnythingOfType("[]*model.WorkerDefinition")).
			Return(expectedWorkers, nil).Once()

		// Ejecutar
		result, err := service.CreateWorkersBatch(ctx, workers)

		// Verificar
		require.NoError(t, err)
		assert.Equal(t, 2, len(result))
		mockRepo.AssertExpectations(t)
		mockIDGen.AssertExpectations(t)
	})

	t.Run("Error de validación en lote", func(t *testing.T) {
		// Preparar - worker inválido (sin nombre)
		workers := []*model.WorkerDefinition{
			createTestWorkerDef("Worker1", model.PENDING),
			createTestWorkerDef("", model.PENDING), // Worker inválido
		}
		workers[1].Metadata.Name = "" // Asegurar que el nombre está vacío

		// No configuramos ninguna expectativa para mockIDGen o mockRepo
		// porque esperamos que falle en la validación

		// Ejecutar
		_, err := service.CreateWorkersBatch(ctx, workers)

		// Verificar
		require.Error(t, err)
		assert.True(t, errors.Is(err, usecases.ErrInvalidWorker))
		mockRepo.AssertNotCalled(t, "BatchSave")

	})
}

func TestWorkerDefinitionService_EliminarWorkerDefinitionsEnLote(t *testing.T) {
	// Preparar
	mockRepo := new(MockRepository)
	mockIDGen := new(MockIDGenerator)
	service := usecases.NewWorkerDefinitionService(mockRepo, mockIDGen)
	ctx := context.Background()

	t.Run("Eliminar lote correctamente", func(t *testing.T) {
		// Preparar
		ids := []model.AggregateID{"id1", "id2", "id3"}

		mockRepo.On("BatchDelete", ctx, ids).Return(nil).Once()

		// Ejecutar
		err := service.DeleteWorkersBatch(ctx, ids)

		// Verificar
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error al eliminar lote", func(t *testing.T) {
		// Preparar
		ids := []model.AggregateID{"id1", "id2"}
		expectedError := errors.New("error eliminando lote")

		mockRepo.On("BatchDelete", ctx, ids).Return(expectedError).Once()

		// Ejecutar
		err := service.DeleteWorkersBatch(ctx, ids)

		// Verificar
		require.Error(t, err)
		assert.Equal(t, expectedError, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestWorkerDefinitionService_FindWorkerDefinitionByName(t *testing.T) {
	// Preparar
	mockRepo := new(MockRepository)
	mockIDGen := new(MockIDGenerator)
	service := usecases.NewWorkerDefinitionService(mockRepo, mockIDGen)
	ctx := context.Background()

	t.Run("Encontrar worker por nombre existente", func(t *testing.T) {
		// Preparar
		workerName := "TestWorker"
		expectedWorker := createTestWorkerDef(workerName, model.HEALTHY)
		expectedResult := ports.SearchResult[*model.WorkerDefinition]{
			Content:       []*model.WorkerDefinition{expectedWorker},
			TotalElements: 1,
			Page:          1,
			Size:          1,
		}

		mockRepo.On("FindByCriteria", ctx, mock.MatchedBy(func(criteria ports.SearchCriteria) bool {
			return criteria.Filters["name"] == workerName &&
				criteria.Page == 1 &&
				criteria.Size == 1
		})).Return(expectedResult, nil).Once()

		// Ejecutar
		result, err := service.FindWorkerDefinitionByName(ctx, workerName)

		// Verificar
		require.NoError(t, err)
		assert.Equal(t, workerName, result.Metadata.Name)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Worker no encontrado", func(t *testing.T) {
		// Preparar
		workerName := "NonExistentWorker"
		emptyResult := ports.SearchResult[*model.WorkerDefinition]{
			Content:       []*model.WorkerDefinition{},
			TotalElements: 0,
			Page:          1,
			Size:          1,
		}

		mockRepo.On("FindByCriteria", ctx, mock.MatchedBy(func(criteria ports.SearchCriteria) bool {
			return criteria.Filters["name"] == workerName
		})).Return(emptyResult, nil).Once()

		// Ejecutar
		result, err := service.FindWorkerDefinitionByName(ctx, workerName)

		// Verificar
		assert.Error(t, err)
		assert.Equal(t, usecases.ErrWorkerNotFound, err)
		assert.Nil(t, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error con nombre vacío", func(t *testing.T) {
		// Ejecutar
		result, err := service.FindWorkerDefinitionByName(ctx, "")

		// Verificar
		assert.Error(t, err)
		assert.Equal(t, usecases.ErrInvalidWorker, err)
		assert.Nil(t, result)
		mockRepo.AssertNotCalled(t, "FindByCriteria")
	})

	t.Run("Error en el repositorio", func(t *testing.T) {
		// Preparar
		workerName := "ErrorWorker"
		repoError := errors.New("error de base de datos")

		mockRepo.On("FindByCriteria", ctx, mock.MatchedBy(func(criteria ports.SearchCriteria) bool {
			return criteria.Filters["name"] == workerName
		})).Return(ports.SearchResult[*model.WorkerDefinition]{}, repoError).Once()

		// Ejecutar
		result, err := service.FindWorkerDefinitionByName(ctx, workerName)

		// Verificar
		assert.Error(t, err)
		assert.Equal(t, repoError, err)
		assert.Nil(t, result)
		mockRepo.AssertExpectations(t)
	})
}
