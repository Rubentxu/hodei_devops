package task_execution_repository_test

import (
	"context"
	generator_id "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository"
	repository "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/execution"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"testing"
	"time"
)

func setupMongo(t *testing.T) (testcontainers.Container, *mongo.Client, *mongo.Database, func()) {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "mongo:5.0",
		ExposedPorts: []string{"27017/tcp"},
		Env:          map[string]string{"MONGO_INITDB_DATABASE": "hodei-test"},
		WaitingFor:   wait.ForLog("Waiting for connections").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, "27017")
	require.NoError(t, err)
	hostIP, err := container.Host(ctx)
	require.NoError(t, err)

	connectionURI := fmt.Sprintf("mongodb://%s:%s", hostIP, mappedPort.Port())
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connectionURI))
	require.NoError(t, err)

	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err = client.Ping(pingCtx, nil)
		cancel()
		if err == nil {
			break
		}
		if i == maxRetries-1 {
			require.NoError(t, err, "No se pudo conectar a MongoDB")
		}
		time.Sleep(time.Second)
	}

	db := client.Database("hodei-test")
	cleanup := func() {
		db.Drop(ctx)
		client.Disconnect(ctx)
		container.Terminate(ctx)
	}

	return container, client, db, cleanup
}

func createTestTaskExecution(name string) *model.TaskExecution {
	now := time.Now().UTC()
	return &model.TaskExecution{
		Metadata: model.Metadata{
			Name:        name,
			Description: "Descripción de " + name,
			Labels:      []string{"test", name},
			Annotations: map[string]string{"env": "test"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Task: model.Task{
			Metadata: model.NewMetadata(
				"Task "+name,
				"Descripción de la tarea "+name,
			),
		},
		Status: model.ExecutionStatus{
			State:     model.Pending,
			StartTime: now,
			EndTime:   now,
			Message:   "Test execution",
		},
		InputArgs: []string{"arg1", "arg2"},
		WorkerDef: model.WorkerDefinition{
			ID: generator_id.NewIDGenerator("").NewID(),
			Metadata: model.NewMetadata(
				"Worker "+name,
				"Descripción del worker "+name,
			),
		},
	}
}

func TestTaskExecutionMongoDBRepository(t *testing.T) {
	_, _, db, cleanup := setupMongo(t)
	defer cleanup()
	ctx := context.Background()
	generator := generator_id.NewIDGenerator("")
	repo := repository.NewTaskExecutionMongoDBRepository(db, generator)

	t.Cleanup(func() {
		db.Collection(repository.TaskExecutionCollection).DeleteMany(ctx, bson.M{})
	})

	t.Run("CRUD Completo", func(t *testing.T) {
		execution := createTestTaskExecution("CRUD Test")

		saved, err := repo.Save(ctx, execution)
		require.NoError(t, err)
		require.NotEqual(t, model.AggregateID(""), saved.ID)

		found, err := repo.FindByID(ctx, saved.ID)
		require.NoError(t, err)
		assert.Equal(t, saved.ID, found.ID)
		assert.Equal(t, execution.Metadata.Name, found.Metadata.Name)

		saved.Status.Message = "Mensaje actualizado"
		err = repo.Update(ctx, saved)
		require.NoError(t, err)

		updated, err := repo.FindByID(ctx, saved.ID)
		require.NoError(t, err)
		assert.Equal(t, "Mensaje actualizado", updated.Status.Message)

		err = repo.Delete(ctx, saved.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, saved.ID)
		assert.ErrorIs(t, err, generic.ErrNotFound)
	})

	t.Run("FindByCriteria avanzado", func(t *testing.T) {
		_, err := db.Collection(repository.TaskExecutionCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		executions := []*model.TaskExecution{
			createTestTaskExecution("Prod-Task"),
			createTestTaskExecution("Dev-Task"),
			createTestTaskExecution("Test-Task"),
		}
		executions[0].Status.State = model.Completed
		executions[1].Status.State = model.Running
		executions[2].Status.State = model.Pending

		// Verificar el estado antes de guardar
		t.Logf("Estado a guardar: %v", executions[0].Status.State)

		var firtsId = ""
		for index, e := range executions {
			saved, err := repo.Save(ctx, e)
			require.NoError(t, err)
			if index == 0 {
				firtsId = saved.WorkerDef.ID.String()
			}

			// Verificar que se guardó correctamente
			found, err := repo.FindByID(ctx, saved.ID)
			require.NoError(t, err)
			assert.Equal(t, e.Status.State, found.Status.State)
		}

		tests := []struct {
			name     string
			criteria ports.SearchCriteria
			expected int
		}{
			{
				"Por estado",
				ports.SearchCriteria{Filters: map[string]interface{}{"state": model.Completed}},
				1,
			},
			{
				"Contiene 'Task' en nombre",
				ports.SearchCriteria{Filters: map[string]interface{}{"nameContains": "Task"}},
				3,
			},
			{
				"Por worker ID",
				ports.SearchCriteria{Filters: map[string]interface{}{"workerdefId": firtsId}},
				1,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := repo.FindByCriteria(ctx, tt.criteria)
				require.NoError(t, err)
				if len(result.Content) != tt.expected {
					t.Logf("Resultados encontrados: %+v", result.Content)
					t.Logf("Filtros aplicados: %+v", tt.criteria.Filters)
				}
				assert.Equal(t, tt.expected, len(result.Content))
			})
		}
	})

	t.Run("Paginación", func(t *testing.T) {
		_, err := db.Collection(repository.TaskExecutionCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		for i := 1; i <= 5; i++ {
			execution := createTestTaskExecution(fmt.Sprintf("Execution %d", i))
			_, err := repo.Save(ctx, execution)
			require.NoError(t, err)
		}

		tests := []struct {
			page     int
			size     int
			expected int
		}{
			{1, 2, 2},
			{2, 2, 2},
			{3, 2, 1},
		}

		for _, tt := range tests {
			t.Run(fmt.Sprintf("Page %d Size %d", tt.page, tt.size), func(t *testing.T) {
				criteria := ports.SearchCriteria{
					Page: tt.page,
					Size: tt.size,
				}
				result, err := repo.FindByCriteria(ctx, criteria)
				require.NoError(t, err)
				assert.Equal(t, tt.expected, len(result.Content))
			})
		}
	})

	t.Run("Operaciones por lotes", func(t *testing.T) {
		_, err := db.Collection(repository.TaskExecutionCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		executions := []*model.TaskExecution{
			createTestTaskExecution("Batch1"),
			createTestTaskExecution("Batch2"),
			createTestTaskExecution("Batch3"),
		}

		savedExecutions, err := repo.BatchSave(ctx, executions)
		require.NoError(t, err)
		assert.Equal(t, len(executions), len(savedExecutions))

		for _, e := range savedExecutions {
			e.Status.Message = "Updated batch"
		}
		err = repo.BatchUpdate(ctx, savedExecutions)
		require.NoError(t, err)

		var ids []model.AggregateID
		for _, e := range savedExecutions {
			ids = append(ids, e.ID)
		}
		err = repo.BatchDelete(ctx, ids)
		require.NoError(t, err)

		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("Concurrencia", func(t *testing.T) {
		execution := createTestTaskExecution("Concurrent")
		saved, err := repo.Save(ctx, execution)
		require.NoError(t, err)

		errCh := make(chan error, 2)
		update := func() {
			e, _ := repo.FindByID(ctx, saved.ID)
			e.Status.Message = uuid.New().String()
			errCh <- repo.Update(context.Background(), e)
		}
		go update()
		go update()

		err1 := <-errCh
		err2 := <-errCh
		assert.NoError(t, err1)
		assert.NoError(t, err2)
	})
}
