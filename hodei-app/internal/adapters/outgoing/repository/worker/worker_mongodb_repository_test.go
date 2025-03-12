package workerdef_repository_test

import (
	"context"
	"testing"
	"time"

	repository "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/worker"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupMongoWorkersWithInitScript(t *testing.T) (testcontainers.Container, *mongo.Client, *mongo.Database, func()) {
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

	// Configurar la colección workers con índice único en id
	_, err = db.Collection("workers").Indexes().CreateOne(ctx, mongo.IndexModel{
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

// Función para crear un WorkerDefinition de prueba
func createTestWorker(name string, instanceType model.InstanceType) *model.WorkerDefinition {
	worker := &model.WorkerDefinition{
		Metadata: model.Metadata{
			Name:        name,
			Description: "Test worker description for " + name,
			Labels:      []string{"test", name},
			Annotations: map[string]string{"env": "test", "purpose": "testing"},
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		},
		Spec: model.WorkerSpec{
			Type:       instanceType,
			Image:      "testimage:latest",
			Env:        map[string]string{"ENV_VAR": "value"},
			WorkingDir: "/app",
			Resources: model.ResourceRequirements{
				CPU:    1.0,
				Memory: "1Gi",
			},
			Volumes: []model.VolumeMount{
				{
					HostPath:      "/host/path",
					ContainerPath: "/container/path",
					ReadOnly:      true,
				},
			},
			Ports: []model.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "TCP",
				},
			},
			Labels: map[string]string{"app": "test"},
			HealthCheck: &model.HealthCheckConfig{
				Type:     "http",
				Endpoint: "/health",
				Interval: 30 * time.Second,
				Timeout:  5 * time.Second,
			},
			TemplateID: "template-123",
		},
		Status: model.WorkerStatus{
			InstanceID: "instance-123",
			Status:     model.HEALTHY, // Usar la constante enum en lugar de string
		},
	}
	return worker
}

func TestWorkerMongoDBRepository(t *testing.T) {
	// Preparar entorno con MongoDB
	_, client, db, cleanup := setupMongoWorkersWithInitScript(t)
	defer cleanup()

	ctx := context.Background()
	repo := repository.NewWorkerMongoDBRepository(db, client)

	// Limpiar cualquier dato existente
	_, err := db.Collection("workers").DeleteMany(ctx, bson.M{})
	require.NoError(t, err)

	t.Run("Guardar y recuperar WorkerDef", func(t *testing.T) {
		// Crear Worker de prueba
		worker := createTestWorker("Test MongoDB Worker", model.DockerInstance)

		// Guardar
		err := repo.Save(ctx, worker)
		require.NoError(t, err)

		// Recuperar
		retrieved, err := repo.FindByID(ctx, worker.ID)
		require.NoError(t, err)

		// Verificar campos
		assert.Equal(t, worker.ID, retrieved.ID)
		assert.Equal(t, worker.Metadata.Name, retrieved.Metadata.Name)
		assert.Equal(t, worker.Metadata.Description, retrieved.Metadata.Description)
		assert.ElementsMatch(t, worker.Metadata.Labels, retrieved.Metadata.Labels)
		assert.Equal(t, worker.Metadata.Annotations["env"], retrieved.Metadata.Annotations["env"])
		assert.Equal(t, worker.Spec.Type, retrieved.Spec.Type)
		assert.Equal(t, worker.Spec.Image, retrieved.Spec.Image)
		assert.Equal(t, worker.Spec.WorkingDir, retrieved.Spec.WorkingDir)
		assert.Equal(t, worker.Spec.Resources.CPU, retrieved.Spec.Resources.CPU)
		assert.Equal(t, worker.Spec.Resources.Memory, retrieved.Spec.Resources.Memory)
		assert.Equal(t, len(worker.Spec.Volumes), len(retrieved.Spec.Volumes))
		assert.Equal(t, worker.Spec.Volumes[0].HostPath, retrieved.Spec.Volumes[0].HostPath)
		assert.Equal(t, worker.Status.InstanceID, retrieved.Status.InstanceID)
		assert.Equal(t, worker.Status.Status, retrieved.Status.Status)
	})

	t.Run("Actualizar WorkerDef", func(t *testing.T) {
		// Crear y guardar un Worker
		worker := createTestWorker("MongoDB Worker para actualizar", model.KubernetesInstance)

		err := repo.Save(ctx, worker)
		require.NoError(t, err)

		// Modificar y actualizar
		worker.Metadata.Description = "Descripción actualizada en MongoDB"
		worker.Metadata.Labels = append(worker.Metadata.Labels, "updated")
		worker.Metadata.Annotations["updated"] = "true"
		worker.Spec.Image = "updated-image:latest"
		worker.Spec.Env["NEW_VAR"] = "new_value"
		worker.Status.Status = model.STOPPED // Usar la constante enum en lugar de string

		err = repo.Update(ctx, worker)
		require.NoError(t, err)

		// Verificar
		retrieved, err := repo.FindByID(ctx, worker.ID)
		require.NoError(t, err)
		assert.Equal(t, "Descripción actualizada en MongoDB", retrieved.Metadata.Description)
		assert.Contains(t, retrieved.Metadata.Labels, "updated")
		assert.Equal(t, "true", retrieved.Metadata.Annotations["updated"])
		assert.Equal(t, "updated-image:latest", retrieved.Spec.Image)
		assert.Equal(t, "new_value", retrieved.Spec.Env["NEW_VAR"])
		assert.Equal(t, model.STOPPED, retrieved.Status.Status)
	})

	t.Run("Eliminar WorkerDef", func(t *testing.T) {
		worker := createTestWorker("MongoDB Worker para eliminar", model.VMInstance)

		// Guardar y eliminar
		err := repo.Save(ctx, worker)
		require.NoError(t, err)

		err = repo.Delete(ctx, worker.ID)
		require.NoError(t, err)

		// Verificar eliminación
		exists, err := repo.Exists(ctx, worker.ID)
		require.NoError(t, err)
		assert.False(t, exists)

		// Intentar recuperar debe fallar
		_, err = repo.FindByID(ctx, worker.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no encontrado")
	})

	t.Run("FindAll debe retornar todos los Workers Def", func(t *testing.T) {
		// Limpiar datos previos
		_, err := db.Collection("workers").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Crear varios Workers
		workers := []*model.WorkerDefinition{
			createTestWorker("MongoDB Worker 1", model.DockerInstance),
			createTestWorker("MongoDB Worker 2", model.KubernetesInstance),
			createTestWorker("MongoDB Worker 3", model.VMInstance),
		}

		for _, worker := range workers {
			err := repo.Save(ctx, worker)
			require.NoError(t, err)
		}

		// Obtener todos los workers
		retrieved, err := repo.FindAll(ctx)
		require.NoError(t, err)
		assert.Equal(t, len(workers), len(retrieved))

		// Verificar que los IDs coincidan (sin importar el orden)
		expectedIDs := make(map[string]bool)
		for _, worker := range workers {
			expectedIDs[worker.ID.String()] = true
		}

		retrievedIDs := make(map[string]bool)
		for _, worker := range retrieved {
			retrievedIDs[worker.ID.String()] = true
		}

		assert.Equal(t, expectedIDs, retrievedIDs)
	})

	t.Run("Búsqueda por criterios múltiples", func(t *testing.T) {
		// Limpiar datos previos
		_, err := db.Collection("workers").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Crear workers con diferentes atributos
		workerDocker := createTestWorker("MongoDB Docker Worker", model.DockerInstance)
		workerDocker.Metadata.Labels = []string{"docker", "dev"}
		workerDocker.Status.Status = model.HEALTHY // Usar la constante enum en lugar de string

		workerK8s := createTestWorker("MongoDB K8s Worker", model.KubernetesInstance)
		workerK8s.Metadata.Labels = []string{"k8s", "prod"}
		workerK8s.Status.Status = model.HEALTHY // Usar la constante enum en lugar de string

		workerVM := createTestWorker("MongoDB VM Worker", model.VMInstance)
		workerVM.Metadata.Labels = []string{"vm", "test"}
		workerVM.Status.Status = model.STOPPED // Usar la constante enum en lugar de string

		workers := []*model.WorkerDefinition{workerDocker, workerK8s, workerVM}
		for _, worker := range workers {
			err := repo.Save(ctx, worker)
			require.NoError(t, err)
		}

		// Test 1: Buscar por tipo
		criteria := ports.SearchCriteria{
			Filters: map[string]interface{}{"type": string(model.DockerInstance)},
			Page:    1,
			Size:    10,
		}

		result, err := repo.FindByCriteria(ctx, criteria)
		require.NoError(t, err)
		assert.Equal(t, int64(1), result.TotalElements)
		assert.Equal(t, "MongoDB Docker Worker", result.Content[0].Metadata.Name)

		// Test 2: Buscar por estado
		criteria = ports.SearchCriteria{
			Filters: map[string]interface{}{"status": "HEALTHY"}, // Usar el string representado por la constante enum
			Page:    1,
			Size:    10,
		}

		result, err = repo.FindByCriteria(ctx, criteria)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.TotalElements)

		// Test 3: Buscar por término en el nombre
		criteria = ports.SearchCriteria{
			Filters: map[string]interface{}{"nameContains": "K8s"},
			Page:    1,
			Size:    10,
		}

		result, err = repo.FindByCriteria(ctx, criteria)
		require.NoError(t, err)
		assert.Equal(t, int64(1), result.TotalElements)
		assert.Equal(t, "MongoDB K8s Worker", result.Content[0].Metadata.Name)
	})

	t.Run("BatchSave y BatchUpdate", func(t *testing.T) {
		// Limpiar datos previos
		_, err := db.Collection("workers").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Crear workers para operaciones por lotes
		workers := []*model.WorkerDefinition{
			createTestWorker("MongoDB Batch Worker 1", model.DockerInstance),
			createTestWorker("MongoDB Batch Worker 2", model.KubernetesInstance),
			createTestWorker("MongoDB Batch Worker 3", model.VMInstance),
		}

		// Guardar por lotes
		err = repo.BatchSave(ctx, workers)
		require.NoError(t, err)

		// Verificar que se hayan guardado todos
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)

		// Modificar todos los workers
		for i := range workers {
			workers[i].Metadata.Description = "Updated in batch with MongoDB"
			workers[i].Status.Status = model.HEALTHY // Usar la constante enum en lugar de string
		}

		// Actualizar por lotes
		err = repo.BatchUpdate(ctx, workers)
		require.NoError(t, err)

		// Verificar las actualizaciones
		for _, worker := range workers {
			retrieved, err := repo.FindByID(ctx, worker.ID)
			require.NoError(t, err)
			assert.Equal(t, "Updated in batch with MongoDB", retrieved.Metadata.Description)
			assert.Equal(t, model.HEALTHY, retrieved.Status.Status)
		}
	})

	t.Run("BatchDelete", func(t *testing.T) {
		// Limpiar datos previos
		_, err := db.Collection("workers").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Crear workers para eliminar por lotes
		workers := []*model.WorkerDefinition{
			createTestWorker("MongoDB Delete Worker 1", model.DockerInstance),
			createTestWorker("MongoDB Delete Worker 2", model.KubernetesInstance),
		}

		// Guardar los workers
		for _, worker := range workers {
			err := repo.Save(ctx, worker)
			require.NoError(t, err)
		}

		// Verificar que se hayan guardado
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)

		// Crear lista de IDs para eliminar
		var ids []model.AggregateID
		for _, worker := range workers {
			ids = append(ids, worker.ID)
		}

		// Eliminar por lotes
		err = repo.BatchDelete(ctx, ids)
		require.NoError(t, err)

		// Verificar que se hayan eliminado
		count, err = repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

}
