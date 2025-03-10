package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	repository "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/resource_pool"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupMongoWithInitScript(t *testing.T) (testcontainers.Container, *mongo.Client, *mongo.Database, func()) {
	ctx := context.Background()

	// Configurar el contenedor MongoDB
	req := testcontainers.ContainerRequest{
		Image:        "mongo:5.0",
		ExposedPorts: []string{"27017/tcp"},
		Env: map[string]string{
			"MONGO_INITDB_DATABASE": "hodei-test",
		},
		WaitingFor: wait.ForLog("Waiting for connections").
			WithStartupTimeout(time.Second * 30),
	}

	// Iniciar el contenedor
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	// Obtener el puerto mapeado y la dirección IP
	mappedPort, err := container.MappedPort(ctx, "27017")
	require.NoError(t, err)

	hostIP, err := container.Host(ctx)
	require.NoError(t, err)

	// Construir la URI de conexión
	connectionURI := "mongodb://" + hostIP + ":" + mappedPort.Port()

	// Crear cliente MongoDB
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connectionURI))
	require.NoError(t, err)

	// Verificar que MongoDB esté listo con múltiples intentos
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err = client.Ping(pingCtx, nil)
		cancel()

		if err == nil {
			break // Conexión exitosa
		}

		if i == maxRetries-1 {
			require.NoError(t, err, "No se pudo conectar a MongoDB después de varios intentos")
		}

		time.Sleep(time.Second) // Esperar antes del siguiente intento
	}

	// Obtener referencia a la base de datos
	db := client.Database("hodei-test")

	// Configurar la colección resource_pools con índice único en id
	_, err = db.Collection("resource_pools").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	require.NoError(t, err)

	// Función de limpieza
	cleanup := func() {
		// Limpiar la base de datos antes de terminar
		_ = db.Drop(ctx)
		_ = client.Disconnect(ctx)
		_ = container.Terminate(ctx)
	}

	return container, client, db, cleanup
}

// Función para crear un ResourcePoolDef de prueba
func createMongoTestResourcePool(name, poolID, poolType string) *model.ResourcePoolDef {
	return &model.ResourcePoolDef{
		ID: model.AggregateID(uuid.New()),
		Metadata: model.Metadata{
			Name:        name,
			Description: "Test Description for " + name,
			Labels:      []string{"test", name},
			Annotations: map[string]string{"env": "test", "purpose": "testing"},
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		},
		Spec: model.ResourcePoolSpec{
			PoolID: poolID,
			Type:   poolType,
			ExtendedSpec: map[string]interface{}{
				"config1": "value1",
				"config2": 42,
				"nested": map[string]interface{}{
					"key1": "nestedValue",
				},
			},
		},
		Status: model.ResourcePoolStatus{
			State: "Active",
		},
	}
}

func TestResourcePoolMongoDBRepository(t *testing.T) {
	// Preparar entorno con MongoDB
	_, client, db, cleanup := setupMongoWithInitScript(t)
	defer cleanup()

	ctx := context.Background()
	repo := repository.NewResourcePoolMongoDBRepository(db, client)

	// Limpiar cualquier dato existente
	_, err := db.Collection("resource_pools").DeleteMany(ctx, bson.M{})
	require.NoError(t, err)

	t.Run("Guardar y recuperar ResourcePool", func(t *testing.T) {
		// Crear ResourcePool de prueba
		pool := createMongoTestResourcePool("Test Pool MongoDB", "mongo-pool-1", "Kubernetes")

		// Guardar
		err := repo.Save(ctx, pool)
		require.NoError(t, err)

		// Recuperar
		retrieved, err := repo.FindByID(ctx, pool.ID)
		require.NoError(t, err)

		// Verificar campos
		assert.Equal(t, pool.ID, retrieved.ID)
		assert.Equal(t, pool.Metadata.Name, retrieved.Metadata.Name)
		assert.Equal(t, pool.Metadata.Description, retrieved.Metadata.Description)
		assert.ElementsMatch(t, pool.Metadata.Labels, retrieved.Metadata.Labels)
		assert.Equal(t, pool.Metadata.Annotations["env"], retrieved.Metadata.Annotations["env"])
		assert.Equal(t, pool.Spec.PoolID, retrieved.Spec.PoolID)
		assert.Equal(t, pool.Spec.Type, retrieved.Spec.Type)
		assert.Equal(t, "value1", retrieved.Spec.ExtendedSpec["config1"])
		assert.Equal(t, int32(42), retrieved.Spec.ExtendedSpec["config2"])
		assert.Equal(t, pool.Status.State, retrieved.Status.State)
	})

	t.Run("Actualizar ResourcePool", func(t *testing.T) {
		// Crear y guardar un ResourcePool
		pool := createMongoTestResourcePool("MongoDB Pool para actualizar", "mongo-update-pool", "Docker")

		err := repo.Save(ctx, pool)
		require.NoError(t, err)

		// Modificar y actualizar
		pool.Metadata.Description = "Descripción actualizada en MongoDB"
		pool.Metadata.Labels = append(pool.Metadata.Labels, "updated")
		pool.Metadata.Annotations["updated"] = "true"
		pool.Spec.ExtendedSpec["config3"] = "nuevo valor"
		pool.Status.State = "Maintenance"

		err = repo.Update(ctx, pool)
		require.NoError(t, err)

		// Verificar
		retrieved, err := repo.FindByID(ctx, pool.ID)
		require.NoError(t, err)
		assert.Equal(t, "Descripción actualizada en MongoDB", retrieved.Metadata.Description)
		assert.Contains(t, retrieved.Metadata.Labels, "updated")
		assert.Equal(t, "true", retrieved.Metadata.Annotations["updated"])
		assert.Equal(t, "nuevo valor", retrieved.Spec.ExtendedSpec["config3"])
		assert.Equal(t, "Maintenance", retrieved.Status.State)
	})

	t.Run("Eliminar ResourcePool", func(t *testing.T) {
		pool := createMongoTestResourcePool("MongoDB Pool para eliminar", "mongo-delete-pool", "VM")

		// Guardar y eliminar
		err := repo.Save(ctx, pool)
		require.NoError(t, err)

		err = repo.Delete(ctx, pool.ID)
		require.NoError(t, err)

		// Verificar eliminación
		exists, err := repo.Exists(ctx, pool.ID)
		require.NoError(t, err)
		assert.False(t, exists)

		// Intentar recuperar debe fallar
		_, err = repo.FindByID(ctx, pool.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no encontrado")
	})

	t.Run("FindAll debe retornar todos los ResourcePools", func(t *testing.T) {
		// Limpiar datos previos
		_, err := db.Collection("resource_pools").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Crear varios ResourcePools
		pools := []*model.ResourcePoolDef{
			createMongoTestResourcePool("Mongo Pool 1", "mongo-pool-1", "Kubernetes"),
			createMongoTestResourcePool("Mongo Pool 2", "mongo-pool-2", "Docker"),
			createMongoTestResourcePool("Mongo Pool 3", "mongo-pool-3", "VM"),
		}

		for _, pool := range pools {
			err := repo.Save(ctx, pool)
			require.NoError(t, err)
		}

		// Obtener todos los pools
		retrieved, err := repo.FindAll(ctx)
		require.NoError(t, err)
		assert.Equal(t, len(pools), len(retrieved))

		// Verificar que los IDs coincidan (sin importar el orden)
		expectedIDs := make(map[string]bool)
		for _, pool := range pools {
			expectedIDs[pool.ID.String()] = true
		}

		retrievedIDs := make(map[string]bool)
		for _, pool := range retrieved {
			retrievedIDs[pool.ID.String()] = true
		}

		assert.Equal(t, expectedIDs, retrievedIDs)
	})

	t.Run("Búsqueda por criterios múltiples", func(t *testing.T) {
		// Limpiar datos previos
		_, err := db.Collection("resource_pools").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Crear pools con diferentes atributos
		pools := []*model.ResourcePoolDef{
			func() *model.ResourcePoolDef {
				p := createMongoTestResourcePool("Mongo Dev K8s", "mongo-dev-k8s", "Kubernetes")
				p.Metadata.Labels = []string{"dev", "k8s"}
				p.Status.State = "Active"
				return p
			}(),
			func() *model.ResourcePoolDef {
				p := createMongoTestResourcePool("Mongo Prod K8s", "mongo-prod-k8s", "Kubernetes")
				p.Metadata.Labels = []string{"prod", "k8s"}
				p.Status.State = "Active"
				return p
			}(),
			func() *model.ResourcePoolDef {
				p := createMongoTestResourcePool("Mongo Test Docker", "mongo-test-docker", "Docker")
				p.Metadata.Labels = []string{"dev", "docker"}
				p.Status.State = "Inactive"
				return p
			}(),
		}

		for _, pool := range pools {
			err := repo.Save(ctx, pool)
			require.NoError(t, err)
		}

		// Test 1: Buscar por tipo
		criteria := ports.SearchCriteria{
			Filters: map[string]interface{}{"type": "Kubernetes"},
			Page:    1,
			Size:    10,
		}

		result, err := repo.FindByCriteria(ctx, criteria)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.TotalElements)

		// Test 2: Buscar por estado
		criteria = ports.SearchCriteria{
			Filters: map[string]interface{}{"state": "Active"},
			Page:    1,
			Size:    10,
		}

		result, err = repo.FindByCriteria(ctx, criteria)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.TotalElements)

		// Test 3: Combinar filtros (tipo y estado)
		criteria = ports.SearchCriteria{
			Filters: map[string]interface{}{
				"type":  "Kubernetes",
				"state": "Active",
			},
			Page: 1,
			Size: 10,
		}

		result, err = repo.FindByCriteria(ctx, criteria)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.TotalElements)

		// Test 4: Buscar por término en el nombre
		criteria = ports.SearchCriteria{
			Filters: map[string]interface{}{"nameContains": "Docker"},
			Page:    1,
			Size:    10,
		}

		result, err = repo.FindByCriteria(ctx, criteria)
		require.NoError(t, err)
		assert.Equal(t, int64(1), result.TotalElements)
	})

	t.Run("BatchSave y BatchUpdate", func(t *testing.T) {
		// Limpiar datos previos
		_, err := db.Collection("resource_pools").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Crear pools para operaciones por lotes
		pools := []*model.ResourcePoolDef{
			createMongoTestResourcePool("Mongo Batch Pool 1", "mongo-batch-1", "Docker"),
			createMongoTestResourcePool("Mongo Batch Pool 2", "mongo-batch-2", "Docker"),
			createMongoTestResourcePool("Mongo Batch Pool 3", "mongo-batch-3", "Kubernetes"),
		}

		// Guardar por lotes
		err = repo.BatchSave(ctx, pools)
		require.NoError(t, err)

		// Verificar que se hayan guardado todos
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)

		// Modificar todos los pools
		for i := range pools {
			pools[i].Metadata.Description = "Updated in batch with MongoDB"
			pools[i].Status.State = "Updated"
		}

		// Actualizar por lotes
		err = repo.BatchUpdate(ctx, pools)
		require.NoError(t, err)

		// Verificar las actualizaciones
		for _, pool := range pools {
			retrieved, err := repo.FindByID(ctx, pool.ID)
			require.NoError(t, err)
			assert.Equal(t, "Updated in batch with MongoDB", retrieved.Metadata.Description)
			assert.Equal(t, "Updated", retrieved.Status.State)
		}
	})

	t.Run("BatchDelete", func(t *testing.T) {
		// Limpiar datos previos
		_, err := db.Collection("resource_pools").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Crear pools para eliminar por lotes
		pools := []*model.ResourcePoolDef{
			createMongoTestResourcePool("Mongo Delete Pool 1", "mongo-delete-1", "Docker"),
			createMongoTestResourcePool("Mongo Delete Pool 2", "mongo-delete-2", "Kubernetes"),
		}

		// Guardar los pools
		for _, pool := range pools {
			err := repo.Save(ctx, pool)
			require.NoError(t, err)
		}

		// Verificar que se hayan guardado
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)

		// Crear lista de IDs para eliminar
		var ids []model.AggregateID
		for _, pool := range pools {
			ids = append(ids, pool.ID)
		}

		// Eliminar por lotes
		err = repo.BatchDelete(ctx, ids)
		require.NoError(t, err)

		// Verificar que se hayan eliminado
		count, err = repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("WithTransaction", func(t *testing.T) {
		// Limpiar datos previos
		_, err := db.Collection("resource_pools").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Crear dos pools en una sola transacción exitosa
		err = repo.WithTransaction(ctx, func(txCtx context.Context) error {
			pool1 := createMongoTestResourcePool("Mongo Tx Pool 1", "mongo-tx-1", "Docker")
			pool2 := createMongoTestResourcePool("Mongo Tx Pool 2", "mongo-tx-2", "Kubernetes")

			err := repo.Save(txCtx, pool1)
			if err != nil {
				return err
			}

			return repo.Save(txCtx, pool2)
		})

		require.NoError(t, err)

		// Verificar que ambos se hayan guardado
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)

		// Transacción que debe fallar (rollback automático)
		err = repo.WithTransaction(ctx, func(txCtx context.Context) error {
			pool3 := createMongoTestResourcePool("Mongo Tx Pool 3", "mongo-tx-3", "VM")

			err := repo.Save(txCtx, pool3)
			if err != nil {
				return err
			}

			// Provocar error deliberadamente
			return errors.New("error forzado para rollback")
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error forzado para rollback")

		// Verificar que no se haya agregado ningún pool nuevo
		count, err = repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})
}
