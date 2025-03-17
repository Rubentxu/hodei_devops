package workerdef_repository_test

import (
	"context"
	generator_id "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	repository "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/worker"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"fmt"
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

func setupMongoWorkersWithInitScript(t *testing.T) (*mongo.Database, func()) {
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
			require.NoError(t, err, "No se pudo conectar a MongoDB después de varios intentos")
		}
		time.Sleep(time.Second)
	}

	db := client.Database("hodei-test")

	cleanup := func() {
		db.Drop(ctx)
		client.Disconnect(ctx)
		container.Terminate(ctx)
	}

	return db, cleanup
}

func TestWorkerMongoDBRepository(t *testing.T) {
	db, cleanup := setupMongoWorkersWithInitScript(t)
	defer cleanup()

	ctx := context.Background()
	generator := generator_id.NewIDGenerator("")
	repo := repository.NewWorkerMongoDBRepository(db, generator)

	// Limpiar la colección antes de cada test
	t.Cleanup(func() {
		db.Collection(repository.WorkerCollection).DeleteMany(ctx, bson.M{})
	})

	t.Run("CRUD Completo", func(t *testing.T) {
		worker := createTestWorker("CRUD Test", model.DockerInstance)

		// Save
		saved, err := repo.Save(ctx, worker)
		require.NoError(t, err)
		require.NotEqual(t, model.AggregateID(""), saved.ID)

		// FindByID
		found, err := repo.FindByID(ctx, saved.ID)
		require.NoError(t, err)
		assert.Equal(t, saved.ID, found.ID)
		assert.Equal(t, worker.Metadata.Name, found.Metadata.Name)

		// Update
		saved.Metadata.Description = "Descripción actualizada"
		err = repo.Update(ctx, saved)
		require.NoError(t, err)

		updated, err := repo.FindByID(ctx, saved.ID)
		require.NoError(t, err)
		assert.Equal(t, "Descripción actualizada", updated.Metadata.Description)

		// Delete
		err = repo.Delete(ctx, saved.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, saved.ID)
		assert.ErrorIs(t, err, generic.ErrNotFound)
	})

	t.Run("FindByCriteria avanzado", func(t *testing.T) {
		// Limpiar la colección antes de ejecutar el test
		_, err := db.Collection(repository.WorkerCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)
		workers := []*model.WorkerDefinition{
			createTestWorker("Worker 1", model.DockerInstance),
			createTestWorker("Worker 2", model.KubernetesInstance),
			createTestWorker("Worker 3", model.VMInstance),
		}

		workers[0].Metadata.Labels = []string{"prod", "docker"}
		workers[1].Metadata.Labels = []string{"dev", "k8s"}
		workers[2].Metadata.Labels = []string{"test", "vm"}
		workers[2].Status.State = model.WorkerStateStopped

		for _, w := range workers {
			_, err := repo.Save(ctx, w)
			require.NoError(t, err)
		}

		tests := []struct {
			name          string
			criteria      ports.SearchCriteria
			expectedCount int64
		}{
			{
				name: "Filtrar por tipo",
				criteria: ports.SearchCriteria{
					Filters: map[string]interface{}{"type": string(model.DockerInstance)},
				},
				expectedCount: 1,
			},
			{
				name: "Filtrar por estado",
				criteria: ports.SearchCriteria{
					Filters: map[string]interface{}{"status": "stopped"},
				},
				expectedCount: 1,
			},
			{
				name: "Filtrar por labels",
				criteria: ports.SearchCriteria{
					Filters: map[string]interface{}{"labels": []string{"prod"}},
				},
				expectedCount: 1,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := repo.FindByCriteria(ctx, tt.criteria)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCount, result.TotalElements)
			})
		}
	})

	t.Run("Concurrencia", func(t *testing.T) {
		worker := createTestWorker("Concurrent", model.DockerInstance)
		saved, err := repo.Save(ctx, worker)
		require.NoError(t, err)

		errCh := make(chan error, 2)
		update := func() {
			ctx := context.Background()
			current, err := repo.FindByID(ctx, saved.ID)
			if err != nil {
				errCh <- err
				return
			}
			current.Metadata.Description = "Updated " + time.Now().String()
			errCh <- repo.Update(ctx, current)
		}

		go update()
		go update()

		err1 := <-errCh
		err2 := <-errCh
		assert.True(t, err1 == nil || err2 == nil, "Al menos una actualización debe tener éxito")
	})

	t.Run("Guardar sin ID", func(t *testing.T) {
		worker := createTestWorker("No ID", model.DockerInstance)
		saved, err := repo.Save(ctx, worker)
		require.NoError(t, err)
		assert.NotEqual(t, model.AggregateID(""), saved.ID)

		exists, err := repo.Exists(ctx, saved.ID)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("Guardar duplicado", func(t *testing.T) {
		worker := createTestWorker("Duplicado", model.KubernetesInstance)
		saved, err := repo.Save(ctx, worker)
		require.NoError(t, err)

		duplicate := createTestWorker("Duplicado", model.KubernetesInstance)
		duplicate.ID = saved.ID

		_, err = repo.Save(ctx, duplicate)
		assert.ErrorIs(t, err, generic.ErrDuplicateID)
	})

	t.Run("Paginación", func(t *testing.T) {
		// Limpiar la colección antes de insertar los nuevos documentos

		_, err := db.Collection(repository.WorkerCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		numWorkers := 5
		// Crear y guardar los workers de prueba
		for i := 1; i <= numWorkers; i++ {
			worker := createTestWorker(fmt.Sprintf("Worker %d", i), model.KubernetesInstance)
			_, err := repo.Save(ctx, worker)
			require.NoError(t, err)
		}

		// Verificar que se insertaron exactamente los elementos esperados
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(numWorkers), count, "Número incorrecto de workers insertados")

		tests := []struct {
			page     int
			size     int
			expected int
		}{
			{1, 2, 2},
			{2, 2, 2},
			{3, 2, 1},
			{0, 10, 5}, // Page 0 usa 1
			{-1, 3, 3}, // Page negativo usa 1
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
		_, err := db.Collection(repository.WorkerCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		names := []string{"Charlie", "Alpha", "Bravo"}
		for _, name := range names {
			worker := createTestWorker(name, model.DockerInstance)
			_, err := repo.Save(ctx, worker)
			require.NoError(t, err)
		}

		tests := []struct {
			sortBy    string
			sortOrder string
			expected  []string
		}{
			{"name", "ASC", []string{"Alpha", "Bravo", "Charlie"}},
			{"name", "DESC", []string{"Charlie", "Bravo", "Alpha"}},
			{"createdAt", "ASC", []string{"Charlie", "Alpha", "Bravo"}},
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
				for _, w := range result.Content {
					actual = append(actual, w.Metadata.Name)
				}
				assert.Equal(t, tt.expected, actual)
			})
		}
	})

	t.Run("Batch Operations", func(t *testing.T) {
		_, err := db.Collection(repository.WorkerCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		workers := []*model.WorkerDefinition{
			createTestWorker("Batch1", model.DockerInstance),
			createTestWorker("Batch2", model.DockerInstance),
		}

		// BatchSave
		savedWorkers, err := repo.BatchSave(ctx, workers)
		require.NoError(t, err)
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)

		// BatchUpdate
		for _, w := range savedWorkers {
			w.Metadata.Description = "Updated"
		}
		err = repo.BatchUpdate(ctx, savedWorkers)
		require.NoError(t, err)

		for _, w := range savedWorkers {
			found, err := repo.FindByID(ctx, w.ID)
			require.NoError(t, err)
			assert.Equal(t, "Updated", found.Metadata.Description)
		}

		// BatchDelete
		var ids []model.AggregateID
		for _, w := range savedWorkers {
			ids = append(ids, w.ID)
		}
		err = repo.BatchDelete(ctx, ids)
		require.NoError(t, err)
		count, err = repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})
}

// Función para crear un WorkerDefinition de prueba
// Función para crear un WorkerDefinition de prueba
func createTestWorker(name string, instanceType model.InstanceType) *model.WorkerDefinition {
	now := time.Now().UTC()
	return &model.WorkerDefinition{
		Metadata: model.Metadata{
			Name:        name,
			Description: "Test worker",
			Labels:      []string{"test"},
			Annotations: map[string]string{"env": "test"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Spec: model.WorkerSpec{
			Type: instanceType,
			Containers: []model.Container{
				{
					Name:    "test-container",
					Image:   "test-image:latest",
					Command: []string{"/bin/sh"},
					Args:    []string{"-c", "echo hello"},
					Env: []model.EnvVar{
						{
							Name:  "ENV_VAR",
							Value: "value",
						},
					},
					Resources: model.ResourceRequirements{
						CPU:    1.0,
						Memory: "1Gi",
					},
					Ports: []model.PortMapping{
						{
							ContainerPort: 8080,
							Protocol:      "TCP",
						},
					},
					WorkingDir:      "/app",
					ImagePullPolicy: model.ImagePullIfNotPresent,
				},
			},
			RestartPolicy: model.RestartPolicyAlways,
			NodeSelector: map[string]string{
				"env": "test",
			},
		},
		Status: model.WorkerStatus{
			State:     model.WorkerStateRunning,
			Message:   "Test status",
			HostIP:    "192.168.1.1",
			WorkerIP:  "10.0.0.1",
			QOSClass:  "Guaranteed",
			StartTime: &now,
			ContainerStatuses: []model.ContainerStatus{
				{
					Name:         "test-container",
					Ready:        true,
					RestartCount: 0,
					State: model.ContainerState{
						Running: &model.ContainerStateRunning{
							StartedAt: now,
						},
					},
					Image:       "test-image:latest",
					ImageID:     "sha256:test123",
					ContainerID: "docker://abc123",
				},
			},
		},
	}
}
