// Archivo: hodei-app/internal/adapters/outgoing/repository/task/task_mongodb_repository_test.go
package task_repository_test

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	generator_id "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository"
	repository "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/task"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"testing"
	"time"
)

func setupMongoTasksWithInitScript(t *testing.T) (testcontainers.Container, *mongo.Client, *mongo.Database, func()) {
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

	// Verificar conexión con reintentos
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
		ctx := context.Background()
		if db != nil {
			err := db.Drop(ctx)
			if err != nil {
				t.Logf("Error al eliminar la base de datos: %v", err)
			}
		}
		if client != nil {
			err := client.Disconnect(ctx)
			if err != nil {
				t.Logf("Error al desconectar el cliente: %v", err)
			}
		}
		if container != nil {
			err := container.Terminate(ctx)
			if err != nil {
				t.Logf("Error al terminar el contenedor: %v", err)
			}
		}
	}

	return container, client, db, cleanup
}

// createTestTask crea una Task de prueba utilizando el modelo definido.
func createTestTask(name string) *model.Task {
	return &model.Task{
		Metadata: model.Metadata{
			Name:        name,
			Description: "Descripción de " + name,
			Labels:      []string{"test", name},
			Annotations: map[string]string{"env": "test"},
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		},
		Spec: model.TaskSpec{
			// Se asigna un WorkerDefinitionName ficticio y se provee el comando y parámetros mínimos.
			WorkerDefinitionName: "defaultWorkerDefinition",
			Command:              []string{"echo", name},
			Params:               []model.ParamDefinition{},
			ParamValues:          make(map[string]interface{}),
		},
	}
}

func TestTaskMongoDBRepository(t *testing.T) {
	_, _, db, cleanup := setupMongoTasksWithInitScript(t)
	defer cleanup()
	ctx := context.Background()
	generator := generator_id.NewIDGenerator("")
	repo := repository.NewTaskMongoDBRepository(db, generator)

	// Limpiar la colección antes de cada test.
	t.Cleanup(func() {
		db.Collection(repository.TaskCollection).DeleteMany(ctx, bson.M{})
	})

	t.Run("CRUD Completo", func(t *testing.T) {
		task := createTestTask("CRUD Test")
		// Save
		saved, err := repo.Save(ctx, task)
		require.NoError(t, err)
		require.NotEqual(t, model.AggregateID(""), saved.ID)

		// FindByID
		found, err := repo.FindByID(ctx, task.ID)
		require.NoError(t, err)
		assert.Equal(t, task.ID, found.ID)
		assert.Equal(t, task.Metadata.Name, found.Metadata.Name)

		// Update
		task.Metadata.Description = "Descripción actualizada"
		err = repo.Update(ctx, task)
		require.NoError(t, err)

		updated, err := repo.FindByID(ctx, task.ID)
		require.NoError(t, err)
		assert.Equal(t, "Descripción actualizada", updated.Metadata.Description)

		// Delete
		err = repo.Delete(ctx, task.ID)
		require.NoError(t, err)
		_, err = repo.FindByID(ctx, task.ID)
		assert.Error(t, err)
	})

	t.Run("Guardar sin ID", func(t *testing.T) {
		task := createTestTask("Sin ID")
		task.ID = model.AggregateID("")
		saved, err := repo.Save(ctx, task)
		require.NoError(t, err)
		assert.NotEqual(t, model.AggregateID(""), saved.ID)
		// Se debe asignar un nuevo ID
		assert.NotEqual(t, uuid.Nil, task.ID)

		exists, err := repo.Exists(ctx, task.ID)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("Guardar duplicado", func(t *testing.T) {
		task := createTestTask("Duplicado")
		saved, err := repo.Save(ctx, task)
		require.NoError(t, err)

		duplicate := createTestTask("Duplicado")
		duplicate.ID = saved.ID
		_, err = repo.Save(ctx, duplicate)
		require.ErrorIs(t, err, generic.ErrDuplicateID)
	})

	t.Run("FindByCriteria avanzado", func(t *testing.T) {
		// Limpiar la colección e insertar varias tasks para búsquedas con filtros.
		_, err := db.Collection(repository.TaskCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		tasks := []*model.Task{
			createTestTask("Prod-K8s"),
			createTestTask("Dev-K8s"),
			createTestTask("Test-Docker"),
		}

		// Actualizar Labels para la prueba.
		tasks[0].Metadata.Labels = []string{"prod", "k8s"}
		tasks[1].Metadata.Labels = []string{"dev", "k8s"}
		tasks[2].Metadata.Labels = []string{"test", "docker"}

		for _, task := range tasks {
			_, err := repo.Save(ctx, task)
			require.NoError(t, err)
		}

		tests := []struct {
			nombre   string
			criteria ports.SearchCriteria
			expected int
		}{
			{
				"Contiene 'K8s' en name",
				ports.SearchCriteria{Filters: map[string]interface{}{"nameContains": "K8s"}},
				2,
			},
			{
				"Labels exactos",
				ports.SearchCriteria{Filters: map[string]interface{}{"labels": []string{"prod", "k8s"}}},
				1,
			},
		}

		for _, tt := range tests {
			t.Run(tt.nombre, func(t *testing.T) {
				result, err := repo.FindByCriteria(ctx, tt.criteria)
				require.NoError(t, err)
				assert.Equal(t, tt.expected, len(result.Content))
				assert.Equal(t, int64(tt.expected), result.TotalElements)
			})
		}
	})

	t.Run("Paginación", func(t *testing.T) {
		_, err := db.Collection(repository.TaskCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		for i := 1; i <= 5; i++ {
			task := createTestTask(fmt.Sprintf("Task %d", i))
			_, err = repo.Save(ctx, task)
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

	t.Run("Ordenamiento", func(t *testing.T) {
		_, err := db.Collection(repository.TaskCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		nombres := []string{"Charlie", "Alpha", "Bravo"}
		for _, nombre := range nombres {
			task := createTestTask(nombre)
			_, err = repo.Save(ctx, task)
			require.NoError(t, err)
		}

		tests := []struct {
			sortBy    string
			sortOrder string
			expected  []string
		}{
			{"name", "ASC", []string{"Alpha", "Bravo", "Charlie"}},
			{"name", "DESC", []string{"Charlie", "Bravo", "Alpha"}},
		}

		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s %s", tt.sortBy, tt.sortOrder), func(t *testing.T) {
				criteria := ports.SearchCriteria{
					SortBy:    tt.sortBy,
					SortOrder: tt.sortOrder,
				}
				result, err := repo.FindByCriteria(ctx, criteria)
				require.NoError(t, err)
				var actual []string
				for _, task := range result.Content {
					actual = append(actual, task.Metadata.Name)
				}
				assert.Equal(t, tt.expected, actual)
			})
		}
	})

	t.Run("Batch Operations", func(t *testing.T) {
		_, err := db.Collection(repository.TaskCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		tasks := []*model.Task{
			createTestTask("Batch1"),
			createTestTask("Batch2"),
		}

		// BatchSave
		savedPools, err := repo.BatchSave(ctx, tasks)
		require.NoError(t, err)
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)

		// BatchUpdate
		for _, task := range tasks {
			task.Metadata.Description = "Updated"
		}
		err = repo.BatchUpdate(ctx, savedPools)
		require.NoError(t, err)

		for _, task := range tasks {
			found, err := repo.FindByID(ctx, task.ID)
			require.NoError(t, err)
			assert.Equal(t, "Updated", found.Metadata.Description)
		}

		// BatchDelete
		var ids []model.AggregateID
		for _, task := range tasks {
			ids = append(ids, task.ID)
		}
		err = repo.BatchDelete(ctx, ids)
		require.NoError(t, err)
		count, err = repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("Concurrencia", func(t *testing.T) {
		_, err := db.Collection(repository.TaskCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		task := createTestTask("Concurrente")
		saved, err := repo.Save(ctx, task)
		require.NoError(t, err)

		errCh := make(chan error, 2)
		updateFunc := func() {
			tsk, err := repo.FindByID(ctx, saved.ID)
			if err == nil {
				tsk.Metadata.Description = uuid.New().String()
				err = repo.Update(ctx, tsk)
			}
			errCh <- err
		}

		go updateFunc()
		go updateFunc()

		err1 := <-errCh
		err2 := <-errCh
		require.NoError(t, err1)
		require.NoError(t, err2)

		updated, err := repo.FindByID(ctx, task.ID)
		require.NoError(t, err)
		// Se verifica que la descripción final sea distinta a la original.
		assert.NotEqual(t, task.Metadata.Description, updated.Metadata.Description)
	})
}
