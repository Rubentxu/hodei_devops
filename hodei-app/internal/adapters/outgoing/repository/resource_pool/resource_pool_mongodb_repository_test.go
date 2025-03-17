package rp_repository_test

import (
	"context"
	generator_id "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"

	repository "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/resource_pool"
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

func createTestPool(name string) *model.ResourcePoolDef {
	return &model.ResourcePoolDef{
		Metadata: model.Metadata{
			Name:        name,
			Description: "Descripción de " + name,
			Labels:      map[string]string{"test": name},
			Annotations: map[string]string{"env": "test"},
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		},
		Spec: model.ResourcePoolSpec{
			PoolID: "pool-" + name,
			Type:   "test-type",
			PoolConfig: map[string]interface{}{
				"config": "value",
			},
		},
		Status: model.ResourcePoolStatus{
			State: "Active",
		},
	}
}

func TestResourcePoolMongoDBRepository(t *testing.T) {
	_, _, db, cleanup := setupMongo(t)
	defer cleanup()
	ctx := context.Background()
	generator := generator_id.NewIDGenerator("")
	repo := repository.NewResourcePoolMongoDBRepository(db, generator)

	// Limpiar la colección antes de cada test
	t.Cleanup(func() {
		db.Collection(repository.ResourcePoolCollection).DeleteMany(ctx, bson.M{})
	})

	t.Run("CRUD Completo", func(t *testing.T) {
		pool := createTestPool("CRUD Test")

		// Save: se captura la entidad devuelta con el ID asignado
		saved, err := repo.Save(ctx, pool)
		require.NoError(t, err)
		require.NotEqual(t, model.AggregateID(""), saved.ID)

		// FindByID
		found, err := repo.FindByID(ctx, saved.ID)
		require.NoError(t, err)
		assert.Equal(t, saved.ID, found.ID)
		assert.Equal(t, pool.Metadata.Name, found.Metadata.Name)

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

	t.Run("Guardar sin ID", func(t *testing.T) {
		pool := createTestPool("No ID")
		saved, err := repo.Save(ctx, pool)
		require.NoError(t, err)
		assert.NotEqual(t, model.AggregateID(""), saved.ID)

		exists, err := repo.Exists(ctx, saved.ID)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("Guardar duplicado", func(t *testing.T) {
		pool := createTestPool("Duplicado")
		saved, err := repo.Save(ctx, pool)
		require.NoError(t, err)

		duplicate := createTestPool("Duplicado")
		duplicate.ID = saved.ID

		_, err = repo.Save(ctx, duplicate)
		assert.ErrorIs(t, err, generic.ErrDuplicateID)
	})

	t.Run("FindByCriteria avanzado", func(t *testing.T) {
		// Limpiar la colección antes de ejecutar el test
		_, err := db.Collection(repository.ResourcePoolCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		pools := []*model.ResourcePoolDef{
			createTestPool("Prod-K8s"),
			createTestPool("Dev-K8s"),
			createTestPool("Test-Docker"),
		}
		pools[0].Metadata.Labels = map[string]string{"prod": "true", "k8s": "true"}
		pools[1].Metadata.Labels = map[string]string{"dev": "true", "k8s": "true"}
		pools[2].Metadata.Labels = map[string]string{"test": "true", "docker": "true"}
		pools[2].Status.State = "Inactive"

		for _, p := range pools {
			_, err := repo.Save(ctx, p)
			require.NoError(t, err)
		}

		tests := []struct {
			name     string
			criteria ports.SearchCriteria
			expected int
		}{
			{
				"Por tipo",
				ports.SearchCriteria{Filters: map[string]interface{}{"type": "test-type"}},
				3,
			},
			{
				"Estado activo",
				ports.SearchCriteria{Filters: map[string]interface{}{"state": "Active"}},
				2,
			},
			{
				"Contiene 'K8s' en nombre",
				ports.SearchCriteria{Filters: map[string]interface{}{"nameContains": "K8s"}},
				2,
			},
			{
				"Filtro combinado",
				ports.SearchCriteria{
					Filters: map[string]interface{}{
						"type":  "test-type",
						"state": "Active",
					},
				},
				2,
			},
			{
				"Labels exactos",
				ports.SearchCriteria{
					Filters: map[string]interface{}{
						"labels": []string{"prod", "k8s"},
					},
				},
				1,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := repo.FindByCriteria(ctx, tt.criteria)
				require.NoError(t, err)
				assert.Equal(t, tt.expected, len(result.Content))
				assert.Equal(t, int64(tt.expected), result.TotalElements)
			})
		}
	})

	t.Run("Paginación", func(t *testing.T) {
		// Limpiar la colección antes de insertar los nuevos documentos
		_, err := db.Collection(repository.ResourcePoolCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		for i := 1; i <= 5; i++ {
			pool := createTestPool(fmt.Sprintf("Pool %d", i))
			_, err := repo.Save(ctx, pool)
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
		// Limpiar la colección para que el test sólo considere los documentos insertados a partir de aquí
		_, err := db.Collection(repository.ResourcePoolCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		names := []string{"Charlie", "Alpha", "Bravo"}
		for _, name := range names {
			pool := createTestPool(name)
			_, err := repo.Save(ctx, pool)
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
				for _, p := range result.Content {
					actual = append(actual, p.Metadata.Name)
				}
				assert.Equal(t, tt.expected, actual)
			})
		}
	})

	t.Run("Batch Operations", func(t *testing.T) {
		// Limpiar la colección para que sólo se consideren los documentos de este test
		_, err := db.Collection(repository.ResourcePoolCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		pools := []*model.ResourcePoolDef{
			createTestPool("Batch1"),
			createTestPool("Batch2"),
		}

		// BatchSave
		savedPools, err := repo.BatchSave(ctx, pools)
		require.NoError(t, err)
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)

		// BatchUpdate
		for _, p := range savedPools {
			p.Metadata.Description = "Updated"
		}
		err = repo.BatchUpdate(ctx, savedPools)
		require.NoError(t, err)

		for _, p := range savedPools {
			found, err := repo.FindByID(ctx, p.ID)
			require.NoError(t, err)
			assert.Equal(t, "Updated", found.Metadata.Description)
		}

		// BatchDelete
		var ids []model.AggregateID
		for _, p := range savedPools {
			ids = append(ids, p.ID)
		}
		err = repo.BatchDelete(ctx, ids)
		require.NoError(t, err)
		count, err = repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("Concurrencia", func(t *testing.T) {
		pool := createTestPool("Concurrent")
		saved, err := repo.Save(ctx, pool)
		require.NoError(t, err)

		errCh := make(chan error, 2)
		update := func() {
			p, _ := repo.FindByID(ctx, saved.ID)
			p.Metadata.Description = uuid.New().String()
			errCh <- repo.Update(context.Background(), p)
		}
		go update()
		go update()

		err1 := <-errCh
		err2 := <-errCh
		assert.NoError(t, err1)
		assert.NoError(t, err2)

		updated, err := repo.FindByID(ctx, saved.ID)
		require.NoError(t, err)
		assert.NotEqual(t, pool.Metadata.Description, updated.Metadata.Description)
	})
}
