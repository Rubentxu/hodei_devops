package repository_test

import (
	"context"
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

// setupMongo configura un contenedor MongoDB para pruebas
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
		db.Drop(ctx)
		client.Disconnect(ctx)
		container.Terminate(ctx)
	}

	return container, client, db, cleanup
}

// createTestPool crea un ResourcePool de prueba con valores predefinidos
func createTestPool(name string) *model.ResourcePoolDef {
	return &model.ResourcePoolDef{
		ID: model.AggregateID(uuid.New()),
		Metadata: model.Metadata{
			Name:        name,
			Description: "Descripción de " + name,
			Labels:      []string{"test", name},
			Annotations: map[string]string{"env": "test"},
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		},
		Spec: model.ResourcePoolSpec{
			PoolID: "pool-" + name,
			Type:   "test-type",
			ExtendedSpec: map[string]interface{}{
				"config": "value",
			},
		},
		Status: model.ResourcePoolStatus{
			State: "Active",
		},
	}
}

// TestResourcePoolMongoDBRepository contiene los casos de prueba
func TestResourcePoolMongoDBRepository(t *testing.T) {
	_, client, db, cleanup := setupMongo(t)
	defer cleanup()
	ctx := context.Background()
	repo := repository.NewResourcePoolMongoDBRepository(db, client)

	// Limpiar colección antes de cada test
	t.Cleanup(func() {
		db.Collection("resource_pools").DeleteMany(ctx, bson.M{})
	})

	t.Run("CRUD Completo", func(t *testing.T) {
		pool := createTestPool("CRUD Test")

		// Save
		err := repo.Save(ctx, pool)
		require.NoError(t, err)

		// FindByID
		found, err := repo.FindByID(ctx, pool.ID)
		require.NoError(t, err)
		assert.Equal(t, pool.ID, found.ID)
		assert.Equal(t, pool.Metadata.Name, found.Metadata.Name)

		// Update
		pool.Metadata.Description = "Descripción actualizada"
		err = repo.Update(ctx, pool)
		require.NoError(t, err)

		updated, err := repo.FindByID(ctx, pool.ID)
		require.NoError(t, err)
		assert.Equal(t, "Descripción actualizada", updated.Metadata.Description)

		// Delete
		err = repo.Delete(ctx, pool.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, pool.ID)
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("Guardar sin ID", func(t *testing.T) {
		pool := createTestPool("No ID")
		pool.ID = model.AggregateID(uuid.Nil)

		err := repo.Save(ctx, pool)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, pool.ID)

		exists, err := repo.Exists(ctx, pool.ID)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("Guardar duplicado", func(t *testing.T) {
		pool := createTestPool("Duplicado")
		err := repo.Save(ctx, pool)
		require.NoError(t, err)

		duplicate := createTestPool("Duplicado")
		duplicate.ID = pool.ID

		err = repo.Save(ctx, duplicate)
		assert.ErrorIs(t, err, repository.ErrDuplicateID)
	})

	t.Run("FindByCriteria avanzado", func(t *testing.T) {
		// Limpiar la colección antes de ejecutar el test
		_, err := db.Collection("resource_pools").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		pools := []*model.ResourcePoolDef{
			createTestPool("Prod-K8s"),
			createTestPool("Dev-K8s"),
			createTestPool("Test-Docker"),
		}

		pools[0].Metadata.Labels = []string{"prod", "k8s"}
		pools[1].Metadata.Labels = []string{"dev", "k8s"}
		pools[2].Metadata.Labels = []string{"test", "docker"}
		pools[2].Status.State = "Inactive"

		for _, p := range pools {
			err := repo.Save(ctx, p)
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

	// Paginación
	t.Run("Paginación", func(t *testing.T) {
		// Limpiar la colección antes de insertar los nuevos documentos
		_, err := db.Collection("resource_pools").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		for i := 1; i <= 5; i++ {
			pool := createTestPool(fmt.Sprintf("Pool %d", i))
			err := repo.Save(ctx, pool)
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
			{0, 10, 5}, // Page 0 debe usar 1
			{-1, 3, 3}, // Page negativo debe usar 1
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
				assert.Equal(t, tt.size <= 0 || tt.size > 5, result.HasNext)
			})
		}
	})

	// Ordenamiento
	t.Run("Ordenamiento", func(t *testing.T) {
		// Limpiar la colección para que el test sólo considere los documentos que se inserten a partir de aquí
		_, err := db.Collection("resource_pools").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		names := []string{"Charlie", "Alpha", "Bravo"}
		for _, name := range names {
			pool := createTestPool(name)
			err := repo.Save(ctx, pool)
			require.NoError(t, err)
		}

		tests := []struct {
			sortBy    string
			sortOrder string
			expected  []string
		}{
			{"name", "ASC", []string{"Alpha", "Bravo", "Charlie"}},
			{"name", "DESC", []string{"Charlie", "Bravo", "Alpha"}},
			{"createdAt", "ASC", []string{"Charlie", "Alpha", "Bravo"}}, // Orden de inserción
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

	// Archivo: hodei-app/internal/adapters/outgoing/repository/resource_pool/resource_pool_mongodb_repository_test.go
	// En el subtest "Batch Operations", se limpia la colección al inicio

	t.Run("Batch Operations", func(t *testing.T) {
		// Limpiar la colección para que sólo se consideren los documentos de este test
		_, err := db.Collection("resource_pools").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		pools := []*model.ResourcePoolDef{
			createTestPool("Batch1"),
			createTestPool("Batch2"),
		}

		// BatchSave
		err = repo.BatchSave(ctx, pools)
		require.NoError(t, err)
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)

		// BatchUpdate
		for _, p := range pools {
			p.Metadata.Description = "Updated"
		}
		err = repo.BatchUpdate(ctx, pools)
		require.NoError(t, err)

		for _, p := range pools {
			found, err := repo.FindByID(ctx, p.ID)
			require.NoError(t, err)
			assert.Equal(t, "Updated", found.Metadata.Description)
		}

		// BatchDelete
		var ids []model.AggregateID
		for _, p := range pools {
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
		err := repo.Save(ctx, pool)
		require.NoError(t, err)

		// Simular actualizaciones concurrentes
		errCh := make(chan error, 2)
		update := func() {
			p, _ := repo.FindByID(ctx, pool.ID)
			p.Metadata.Description = uuid.New().String()
			errCh <- repo.Update(context.Background(), p)
		}

		go update()
		go update()

		err1 := <-errCh
		err2 := <-errCh
		assert.NoError(t, err1)
		assert.NoError(t, err2)

		// Verificar estado final
		updated, err := repo.FindByID(ctx, pool.ID)
		require.NoError(t, err)
		assert.NotEqual(t, pool.Metadata.Description, updated.Metadata.Description)
	})
}
