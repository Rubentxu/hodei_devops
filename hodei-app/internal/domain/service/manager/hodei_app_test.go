package manager_test

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/service/manager"
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// Mock para IDGenerator
type MockIDGenerator struct {
	mock.Mock
}

func (m *MockIDGenerator) NewID() model.AggregateID {
	args := m.Called()
	return args.Get(0).(model.AggregateID)
}

// Mock para Scheduler
type MockScheduler struct {
	mock.Mock
}

func (m *MockScheduler) SelectCandidateNodes(definition *model.WorkerDefinition, pools []*ports.ResourcePool) []*ports.ResourcePool {
	args := m.Called(definition, pools)
	return args.Get(0).([]*ports.ResourcePool)
}

func (m *MockScheduler) Score(pools []*ports.ResourcePool) map[string]float64 {
	args := m.Called(pools)
	return args.Get(0).(map[string]float64)
}

func (m *MockScheduler) Pick(scores map[string]float64, candidates []*ports.ResourcePool) *ports.ResourcePool {
	args := m.Called(scores, candidates)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*ports.ResourcePool)
}

// Mock para ResourcePoolService
type MockResourcePoolService struct {
	mock.Mock
}

func (m *MockResourcePoolService) ListActivePools() []*ports.ResourcePool {
	args := m.Called()
	return args.Get(0).([]*ports.ResourcePool)
}

func (m *MockResourcePoolService) GetActivePool(id string) (*ports.ResourcePool, bool) {
	args := m.Called(id)
	return args.Get(0).(*ports.ResourcePool), args.Bool(1)
}

// Implementaciones para otros métodos que no usamos directamente
func (m *MockResourcePoolService) CreateResourcePool(ctx context.Context, resourceDef *model.ResourcePoolDef) (*model.ResourcePoolDef, error) {
	return nil, nil
}
func (m *MockResourcePoolService) UpdateResourcePool(ctx context.Context, id model.AggregateID, updates *model.ResourcePoolDef) error {
	return nil
}
func (m *MockResourcePoolService) DeleteResourcePool(ctx context.Context, id model.AggregateID) error {
	return nil
}
func (m *MockResourcePoolService) GetResourcePool(ctx context.Context, id model.AggregateID) (*model.ResourcePoolDef, error) {
	return nil, nil
}
func (m *MockResourcePoolService) ListResourcePools(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.ResourcePoolDef], error) {
	return ports.SearchResult[*model.ResourcePoolDef]{}, nil
}
func (m *MockResourcePoolService) CreateResourcePoolInstance(ctx context.Context, id model.AggregateID) (*ports.ResourcePool, error) {
	return nil, nil
}
func (m *MockResourcePoolService) CreateAllResourcePools(ctx context.Context) error {
	return nil
}

// Mock para TaskService
type MockTaskService struct {
	mock.Mock
}

func (m *MockTaskService) GetTask(ctx context.Context, id model.AggregateID) (*model.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Task), args.Error(1)
}

// Implementación de otros métodos requeridos por la interfaz
func (m *MockTaskService) CreateTask(ctx context.Context, task *model.Task) (*model.Task, error) {
	return nil, nil
}
func (m *MockTaskService) UpdateTask(ctx context.Context, id model.AggregateID, updates *model.Task) error {
	return nil
}
func (m *MockTaskService) DeleteTask(ctx context.Context, id model.AggregateID) error {
	return nil
}
func (m *MockTaskService) ListTasks(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.Task], error) {
	return ports.SearchResult[*model.Task]{}, nil
}

// Mock para WorkerDefinitionService
type MockWorkerDefService struct {
	mock.Mock
}

func (m *MockWorkerDefService) FindWorkerDefinitionByName(ctx context.Context, name string) (*model.WorkerDefinition, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.WorkerDefinition), args.Error(1)
}

// Implementación de otros métodos requeridos por la interfaz
func (m *MockWorkerDefService) CreateWorkerDefinition(ctx context.Context, workerDef *model.WorkerDefinition) (*model.WorkerDefinition, error) {
	return nil, nil
}
func (m *MockWorkerDefService) GetWorkerDefinition(ctx context.Context, id model.AggregateID) (*model.WorkerDefinition, error) {
	return nil, nil
}
func (m *MockWorkerDefService) UpdateWorkerDefinition(ctx context.Context, updates *model.WorkerDefinition) error {
	return nil
}
func (m *MockWorkerDefService) DeleteWorkerDefinition(ctx context.Context, id model.AggregateID) error {
	return nil
}
func (m *MockWorkerDefService) FindWorkerDefinitions(ctx context.Context, criterio ports.SearchCriteria) (ports.SearchResult[*model.WorkerDefinition], error) {
	return ports.SearchResult[*model.WorkerDefinition]{}, nil
}
func (m *MockWorkerDefService) UpdateWorkerStatus(ctx context.Context, id model.AggregateID, estado model.HealthStatus) error {
	return nil
}
func (m *MockWorkerDefService) AssignTemplate(ctx context.Context, workerID model.AggregateID, templateID string) error {
	return nil
}
func (m *MockWorkerDefService) CreateWorkersBatch(ctx context.Context, workerDefs []*model.WorkerDefinition) ([]*model.WorkerDefinition, error) {
	return nil, nil
}
func (m *MockWorkerDefService) DeleteWorkersBatch(ctx context.Context, ids []model.AggregateID) error {
	return nil
}

// Mock para TaskExecutionService
type MockTaskExecService struct {
	mock.Mock
}

func (m *MockTaskExecService) CreateTaskExecution(ctx context.Context, execution *model.TaskExecution) (*model.TaskExecution, error) {
	args := m.Called(ctx, execution)
	return args.Get(0).(*model.TaskExecution), args.Error(1)
}

func (m *MockTaskExecService) UpdateTaskExecutionStatus(ctx context.Context, id model.AggregateID, status model.ExecutionStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

// Implementación de otros métodos requeridos por la interfaz
func (m *MockTaskExecService) GetTaskExecution(ctx context.Context, id model.AggregateID) (*model.TaskExecution, error) {
	return nil, nil
}
func (m *MockTaskExecService) ListTaskExecutions(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.TaskExecution], error) {
	return ports.SearchResult[*model.TaskExecution]{}, nil
}
func (m *MockTaskExecService) CancelTaskExecution(ctx context.Context, id model.AggregateID) error {
	return nil
}
func (m *MockTaskExecService) GetTaskExecutionMetrics(ctx context.Context) (ports.TaskExecutionMetrics, error) {
	return ports.TaskExecutionMetrics{}, nil
}

// Mock para ResourceIntanceClient
type MockResourceIntanceClient struct {
	mock.Mock
}

func (m *MockResourceIntanceClient) GetNativeClient() any {
	args := m.Called()
	return args.Get(0)
}

func (m *MockResourceIntanceClient) GetConfig() any {
	args := m.Called()
	return args.Get(0)
}

// Mock para ResourcePool
type MockResourcePool struct {
	mock.Mock
}

func (m *MockResourcePool) GetID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockResourcePool) GetStats() (*model.Stats, error) {
	args := m.Called()
	return args.Get(0).(*model.Stats), args.Error(1)
}

func (m *MockResourcePool) Matches(definition model.WorkerDefinition) bool {
	args := m.Called(definition)
	return args.Bool(0)
}

func (m *MockResourcePool) GetResourceInstanceClient() ports.ResourceIntanceClient {
	args := m.Called()
	return args.Get(0).(ports.ResourceIntanceClient)
}

// Mock para WorkerInstanceManager
type MockWorkerInstanceManager struct {
	mock.Mock
}

func (m *MockWorkerInstanceManager) AddTask(request model.TaskExecutionRequest, ctx context.Context) (ports.TaskContext, error) {
	args := m.Called(request)
	return args.Get(0).(ports.TaskContext), args.Error(1)
}

func (m *MockWorkerInstanceManager) StopTask(taskContext ports.TaskContext) error {
	args := m.Called(taskContext)
	return args.Error(0)
}

func TestNew(t *testing.T) {
	// Mocks
	mockWorkerManager := &MockWorkerInstanceManager{}
	mockPoolService := &MockResourcePoolService{}
	mockTaskService := &MockTaskService{}
	mockWorkerDefService := &MockWorkerDefService{}
	mockTaskExecService := &MockTaskExecService{}
	mockGenerator := &MockIDGenerator{}

	poolServicePtr := ports.ResourcePoolService(mockPoolService)
	taskServicePtr := ports.TaskService(mockTaskService)
	workerDefServicePtr := ports.WorkerDefinitionService(mockWorkerDefService)
	taskExecServicePtr := ports.TaskExecutionService(mockTaskExecService)

	t.Run("Crear HodeiApp con scheduler greedy", func(t *testing.T) {
		app, err := manager.NewHodeiApp("greedy", mockWorkerManager, &poolServicePtr, &taskServicePtr, &workerDefServicePtr, &taskExecServicePtr, 100, mockGenerator)
		require.NoError(t, err)
		require.NotNil(t, app)

	})

	t.Run("Crear HodeiApp con scheduler roundrobin", func(t *testing.T) {
		app, err := manager.NewHodeiApp("roundrobin", mockWorkerManager, &poolServicePtr, &taskServicePtr, &workerDefServicePtr, &taskExecServicePtr, 100, mockGenerator)
		require.NoError(t, err)
		require.NotNil(t, app)

	})

	t.Run("Crear HodeiApp con scheduler por defecto", func(t *testing.T) {
		app, err := manager.NewHodeiApp("unknown", mockWorkerManager, &poolServicePtr, &taskServicePtr, &workerDefServicePtr, &taskExecServicePtr, 100, mockGenerator)
		require.NoError(t, err)
		require.NotNil(t, app)

	})
}

func TestAddTask(t *testing.T) {
	// Setup común
	ctx := context.Background()
	mockWorkerManager := &MockWorkerInstanceManager{}
	mockPoolService := &MockResourcePoolService{}
	mockTaskService := &MockTaskService{}
	mockWorkerDefService := &MockWorkerDefService{}
	mockTaskExecService := &MockTaskExecService{}
	mockGenerator := &MockIDGenerator{}

	poolServicePtr := ports.ResourcePoolService(mockPoolService)
	taskServicePtr := ports.TaskService(mockTaskService)
	workerDefServicePtr := ports.WorkerDefinitionService(mockWorkerDefService)
	taskExecServicePtr := ports.TaskExecutionService(mockTaskExecService)

	t.Run("Añadir tarea válida", func(t *testing.T) {
		// Crear app con buffer pequeño para probar correctamente
		app, _ := manager.NewHodeiApp("greedy", mockWorkerManager, &poolServicePtr, &taskServicePtr, &workerDefServicePtr, &taskExecServicePtr, 10, mockGenerator)

		taskID := model.AggregateID("task-123")
		execID := model.AggregateID("exec-123")
		mockGenerator.On("NewID").Return(execID)

		task := &model.Task{
			ID: taskID,
			Metadata: model.Metadata{
				Name:        "Test Task",
				Description: "Test Description",
			},
			Spec: model.TaskSpec{
				WorkerDefinitionName: "test-worker",
			},
		}

		workerDef := &model.WorkerDefinition{
			ID: model.AggregateID("worker-123"),
			Metadata: model.Metadata{
				Name:        "test-worker",
				Description: "Test Worker",
			},
		}

		request := model.TaskExecutionRequest{
			TaskID: taskID,
		}

		// Configurar mocks
		mockTaskService.On("GetTask", ctx, taskID).Return(task, nil)
		mockWorkerDefService.On("FindWorkerDefinitionByName", ctx, "test-worker").Return(workerDef, nil)

		execution := &model.TaskExecution{
			ID:       execID,
			Metadata: model.NewMetadata("Test Task"+mock.Anything, "Test Description"),
			Status: model.ExecutionStatus{
				StartTime: mock.Anything.(time.Time),
				State:     model.Pending,
			},
			WorkerDef: workerDef,
		}

		mockTaskExecService.On("CreateTaskExecution", ctx, mock.AnythingOfType("*model.TaskExecution")).Return(execution, nil)

		taskContext, err := app.AddTask(request, ctx)

		require.NoError(t, err)
		assert.Equal(t, execID, taskContext.Execution.ID)
		assert.Equal(t, model.Pending, taskContext.Execution.Status.State)

		// Limpiar el canal para siguientes pruebas
		go func() {
			for range app.pendingTasksChan {
				// Vaciar el canal
			}
		}()
	})

	t.Run("Error cuando no se encuentra la tarea", func(t *testing.T) {
		app, _ := manager.NewHodeiApp("greedy", mockWorkerManager, &poolServicePtr, &taskServicePtr, &workerDefServicePtr, &taskExecServicePtr, 10, mockGenerator)

		taskID := model.AggregateID("nonexistent-task")
		request := model.TaskExecutionRequest{TaskID: taskID}

		mockTaskService.On("GetTask", ctx, taskID).Return((*model.Task)(nil), errors.New("tarea no encontrada"))

		_, err := app.AddTask(request, ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tarea no encontrada")
	})

	t.Run("Error cuando no se encuentra la definición del worker", func(t *testing.T) {
		app, _ := manager.NewHodeiApp("greedy", mockWorkerManager, &poolServicePtr, &taskServicePtr, &workerDefServicePtr, &taskExecServicePtr, 10, mockGenerator)

		taskID := model.AggregateID("task-456")
		task := &model.Task{
			ID: taskID,
			Metadata: model.Metadata{
				Name:        "Test Task",
				Description: "Test Description",
			},
			Spec: model.TaskSpec{
				WorkerDefinitionName: "nonexistent-worker",
			},
		}

		request := model.TaskExecutionRequest{TaskID: taskID}
		mockTaskService.On("GetTask", ctx, taskID).Return(task, nil)
		mockWorkerDefService.On("FindWorkerDefinitionByName", ctx, "nonexistent-worker").Return((*model.WorkerDefinition)(nil), errors.New("worker no encontrado"))

		_, err := app.AddTask(request, ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "definición de worker no encontrada")
	})
}

func TestSelectWorker(t *testing.T) {
	// Setup común
	ctx := context.Background()
	mockWorkerManager := &MockWorkerInstanceManager{}
	mockPoolService := &MockResourcePoolService{}
	mockTaskService := &MockTaskService{}
	mockWorkerDefService := &MockWorkerDefService{}
	mockTaskExecService := &MockTaskExecService{}
	mockGenerator := &MockIDGenerator{}
	mockScheduler := &MockScheduler{}

	poolServicePtr := ports.ResourcePoolService(mockPoolService)
	taskServicePtr := ports.TaskService(mockTaskService)
	workerDefServicePtr := ports.WorkerDefinitionService(mockWorkerDefService)
	taskExecServicePtr := ports.TaskExecutionService(mockTaskExecService)

	app, _ := manager.NewHodeiApp("greedy", mockWorkerManager, &poolServicePtr, &taskServicePtr, &workerDefServicePtr, &taskExecServicePtr, 10, mockGenerator)
	// Reemplazar el scheduler por nuestro mock
	app.scheduler = mockScheduler

	workerDef := &model.WorkerDefinition{
		ID: model.AggregateID("worker-123"),
		Metadata: model.Metadata{
			Name: "test-worker",
		},
	}

	t.Run("Seleccionar worker con pools disponibles", func(t *testing.T) {
		// Crear mocks de pools
		mockPool1 := &MockResourcePool{}
		mockPool1.On("GetID").Return("pool-1")

		mockPool2 := &MockResourcePool{}
		mockPool2.On("GetID").Return("pool-2")

		pools := []*ports.ResourcePool{
			(*ports.ResourcePool)(mockPool1),
			(*ports.ResourcePool)(mockPool2),
		}

		candidates := []*ports.ResourcePool{(*ports.ResourcePool)(mockPool1)}
		scores := map[string]float64{"pool-1": 0.9}

		// Configurar comportamiento del scheduler
		mockPoolService.On("ListActivePools").Return(pools)
		mockScheduler.On("SelectCandidateNodes", workerDef, pools).Return(candidates)
		mockScheduler.On("Score", candidates).Return(scores)
		mockScheduler.On("Pick", scores, candidates).Return((*ports.ResourcePool)(mockPool1))

		selectedPool, err := app.SelectWorker(workerDef)

		require.NoError(t, err)
		assert.NotNil(t, selectedPool)
		assert.Equal(t, "pool-1", (*selectedPool).GetID())

		mockPoolService.AssertExpectations(t)
		mockScheduler.AssertExpectations(t)
	})

	t.Run("Error cuando no hay pools disponibles", func(t *testing.T) {
		emptyPools := []*ports.ResourcePool{}

		mockPoolService.On("ListActivePools").Return(emptyPools)

		selectedPool, err := app.SelectWorker(workerDef)

		assert.Error(t, err)
		assert.Nil(t, selectedPool)
		assert.Contains(t, err.Error(), "no hay ResourcePools disponibles")
	})

	t.Run("Error cuando el scheduler no puede seleccionar un pool", func(t *testing.T) {
		mockPool1 := &MockResourcePool{}
		mockPool1.On("GetID").Return("pool-1")

		pools := []*ports.ResourcePool{
			(*ports.ResourcePool)(mockPool1),
		}

		candidates := []*ports.ResourcePool{(*ports.ResourcePool)(mockPool1)}
		scores := map[string]float64{"pool-1": 0.9}

		mockPoolService.On("ListActivePools").Return(pools)
		mockScheduler.On("SelectCandidateNodes", workerDef, pools).Return(candidates)
		mockScheduler.On("Score", candidates).Return(scores)
		mockScheduler.On("Pick", scores, candidates).Return((*ports.ResourcePool)(nil))

		selectedPool, err := app.SelectWorker(workerDef)

		assert.Error(t, err)
		assert.Nil(t, selectedPool)
		assert.Contains(t, err.Error(), "no se pudo seleccionar un ResourcePool")
	})
}

func TestProcessTasks(t *testing.T) {
	// Setup común
	ctx := context.Background()
	mockWorkerManager := &MockWorkerInstanceManager{}
	mockPoolService := &MockResourcePoolService{}
	mockTaskService := &MockTaskService{}
	mockWorkerDefService := &MockWorkerDefService{}
	mockTaskExecService := &MockTaskExecService{}
	mockGenerator := &MockIDGenerator{}
	mockScheduler := &MockScheduler{}

	poolServicePtr := ports.ResourcePoolService(mockPoolService)
	taskServicePtr := ports.TaskService(mockTaskService)
	workerDefServicePtr := ports.WorkerDefinitionService(mockWorkerDefService)
	taskExecServicePtr := ports.TaskExecutionService(mockTaskExecService)

	t.Run("Procesamiento exitoso de una tarea", func(t *testing.T) {
		app, _ := manager.NewHodeiApp("greedy", mockWorkerManager, &poolServicePtr, &taskServicePtr, &workerDefServicePtr, &taskExecServicePtr, 10, mockGenerator)
		app.scheduler = mockScheduler

		// Crear workerDef para la tarea
		workerDef := &model.WorkerDefinition{
			ID: model.AggregateID("worker-123"),
			Metadata: model.Metadata{
				Name: "test-worker",
			},
		}

		// Crear una ejecución de tarea
		execution := model.TaskExecution{
			ID: model.AggregateID("exec-123"),
			Metadata: model.Metadata{
				Name:        "Test Execution",
				Description: "Test Description",
			},
			Status: model.ExecutionStatus{
				StartTime: time.Now(),
				State:     model.Pending,
			},
			WorkerDef: workerDef,
		}

		// Crear un contexto de tarea
		taskContext := ports.TaskContext{
			Execution:  execution,
			OutputChan: make(chan model.ProcessOutput, 10),
			StateChan:  make(chan model.TaskState, 1),
			ErrChan:    make(chan error, 1),
			Ctx:        ctx,
		}

		// Configurar mocks de pool
		mockPool := &MockResourcePool{}
		mockPool.On("GetID").Return("pool-1")

		mockClient := &MockResourceIntanceClient{}
		mockPool.On("GetResourceInstanceClient").Return(mockClient)

		// Configurar comportamiento del scheduler
		pools := []*ports.ResourcePool{(*ports.ResourcePool)(mockPool)}
		candidates := []*ports.ResourcePool{(*ports.ResourcePool)(mockPool)}
		scores := map[string]float64{"pool-1": 0.9}

		mockPoolService.On("ListActivePools").Return(pools)
		mockScheduler.On("SelectCandidateNodes", workerDef, pools).Return(candidates)
		mockScheduler.On("Score", candidates).Return(scores)
		mockScheduler.On("Pick", scores, candidates).Return((*ports.ResourcePool)(mockPool))

		// Configurar el WorkerManager para que ejecute la tarea con éxito
		mockWorkerManager.On("AddTask", mock.AnythingOfType("ports.TaskContext")).Return(nil)

		// Configurar actualización de estado
		mockTaskExecService.On("UpdateTaskExecutionStatus", ctx, model.AggregateID("exec-123"), mock.AnythingOfType("model.ExecutionStatus")).Return(nil)

		// Enviar la tarea al canal y procesar
		app.pendingTasksChan <- taskContext

		// Iniciar procesamiento en una goroutine
		go app.ProcessTasks()

		// Dar tiempo para que se procese
		time.Sleep(100 * time.Millisecond)

		mockPoolService.AssertExpectations(t)
		mockWorkerManager.AssertExpectations(t)
		mockTaskExecService.AssertExpectations(t)
	})
}

func TestStopTask(t *testing.T) {
	// Setup
	mockWorkerManager := &MockWorkerInstanceManager{}
	mockPoolService := &MockResourcePoolService{}
	mockTaskService := &MockTaskService{}
	mockWorkerDefService := &MockWorkerDefService{}
	mockTaskExecService := &MockTaskExecService{}
	mockGenerator := &MockIDGenerator{}

	poolServicePtr := ports.ResourcePoolService(mockPoolService)
	taskServicePtr := ports.TaskService(mockTaskService)
	workerDefServicePtr := ports.WorkerDefinitionService(mockWorkerDefService)
	taskExecServicePtr := ports.TaskExecutionService(mockTaskExecService)

	app, _ := manager.NewHodeiApp("greedy", mockWorkerManager, &poolServicePtr, &taskServicePtr, &workerDefServicePtr, &taskExecServicePtr, 10, mockGenerator)

	t.Run("Detener tarea exitosamente", func(t *testing.T) {
		// Crear un contexto de tarea
		taskContext := ports.TaskContext{
			Execution: model.TaskExecution{
				ID: model.AggregateID("exec-to-stop"),
			},
			Ctx: context.Background(),
		}

		// Configurar el mock para que devuelva éxito
		mockWorkerManager.On("StopTask", taskContext).Return(nil)

		// Intentar detener la tarea
		err := app.StopTask(taskContext)

		assert.NoError(t, err)
		mockWorkerManager.AssertExpectations(t)
	})

	t.Run("Error al detener tarea", func(t *testing.T) {
		taskContext := ports.TaskContext{
			Execution: model.TaskExecution{
				ID: model.AggregateID("exec-error"),
			},
			Ctx: context.Background(),
		}

		// Configurar el mock para que devuelva un error
		expectedErr := errors.New("error deteniendo la tarea")
		mockWorkerManager.On("StopTask", taskContext).Return(expectedErr)

		// Intentar detener la tarea
		err := app.StopTask(taskContext)

		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		mockWorkerManager.AssertExpectations(t)
	})
}

func TestDoHealthChecks(t *testing.T) {
	// Setup
	mockWorkerManager := &MockWorkerInstanceManager{}
	mockPoolService := &MockResourcePoolService{}
	mockTaskService := &MockTaskService{}
	mockWorkerDefService := &MockWorkerDefService{}
	mockTaskExecService := &MockTaskExecService{}
	mockGenerator := &MockIDGenerator{}

	poolServicePtr := ports.ResourcePoolService(mockPoolService)
	taskServicePtr := ports.TaskService(mockTaskService)
	workerDefServicePtr := ports.WorkerDefinitionService(mockWorkerDefService)
	taskExecServicePtr := ports.TaskExecutionService(mockTaskExecService)

	app, _ := manager.NewHodeiApp("greedy", mockWorkerManager, &poolServicePtr, &taskServicePtr, &workerDefServicePtr, &taskExecServicePtr, 10, mockGenerator)

	t.Run("Verificación de salud - método incompleto", func(t *testing.T) {
		// Como DoHealthChecks está incompleto, solo verificamos que no cause errores
		assert.NotPanics(t, func() {
			app.DoHealthChecks()
		})
	})
}
