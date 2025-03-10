package repository_test

import (
	"context"
	"testing"
	"time"

	repository "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/task"
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

func setupMongoTasksWithInitScript(t *testing.T) (testcontainers.Container, *mongo.Client, *mongo.Database, func()) {
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

	// Configurar la colección tasks con índice único en id
	_, err = db.Collection("tasks").Indexes().CreateOne(ctx, mongo.IndexModel{
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

// Función para crear un Task de prueba
func createTestTask(name string) *model.Task {
	// Crear parámetros de ejemplo
	params := []model.ParamDefinition{
		{
			Key:         "param1",
			Type:        model.ParamTypeString,
			Label:       "Parameter 1",
			Description: "First parameter for testing",
			Required:    true,
			Default:     "default value",
			Group:       "test",
			Order:       1,
		},
		{
			Key:         "param2",
			Type:        model.ParamTypeInteger,
			Label:       "Parameter 2",
			Description: "Second parameter for testing",
			Required:    false,
			Default:     42,
			Group:       "test",
			Order:       2,
			Validations: model.ParamValidations{
				Min: func() *float64 { val := float64(1); return &val }(),
				Max: func() *float64 { val := float64(100); return &val }(),
			},
		},
	}

	// Crear una Task con valores de prueba
	task := &model.Task{
		ID: model.AggregateID(uuid.New()),
		Metadata: model.Metadata{
			Name:        name,
			Description: "Test task description for " + name,
			Labels:      []string{"test", name},
			Annotations: map[string]string{"env": "test", "purpose": "testing"},
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		},
		Spec: model.TaskSpec{
			WorkerDefinitionID: model.AggregateID(uuid.New()),
			Command:            []string{"echo", "Hello World"},
			Params:             params,
			ParamValues: map[string]interface{}{
				"param1": "test value",
				"param2": 50,
			},
		},
	}

	return task
}

func TestTaskMongoDBRepository(t *testing.T) {
	// Preparar entorno con MongoDB
	_, client, db, cleanup := setupMongoTasksWithInitScript(t)
	defer cleanup()

	ctx := context.Background()
	repo := repository.NewTaskMongoDBRepository(db, client)

	// Limpiar cualquier dato existente
	_, err := db.Collection("tasks").DeleteMany(ctx, bson.M{})
	require.NoError(t, err)

	t.Run("Guardar y recuperar Task", func(t *testing.T) {
		// Crear Task de prueba
		task := createTestTask("Test MongoDB Task")

		// Guardar
		err := repo.Save(ctx, task)
		require.NoError(t, err)

		// Recuperar
		retrieved, err := repo.FindByID(ctx, task.ID)
		require.NoError(t, err)

		// Verificar campos
		assert.Equal(t, task.ID, retrieved.ID)
		assert.Equal(t, task.Metadata.Name, retrieved.Metadata.Name)
		assert.Equal(t, task.Metadata.Description, retrieved.Metadata.Description)
		assert.ElementsMatch(t, task.Metadata.Labels, retrieved.Metadata.Labels)
		assert.Equal(t, task.Metadata.Annotations["env"], retrieved.Metadata.Annotations["env"])
		assert.Equal(t, task.Spec.WorkerDefinitionID, retrieved.Spec.WorkerDefinitionID)
		assert.ElementsMatch(t, task.Spec.Command, retrieved.Spec.Command)

		// Verificar parámetros
		assert.Equal(t, len(task.Spec.Params), len(retrieved.Spec.Params))
		assert.Equal(t, task.Spec.Params[0].Key, retrieved.Spec.Params[0].Key)
		assert.Equal(t, task.Spec.Params[0].Type, retrieved.Spec.Params[0].Type)
		assert.Equal(t, task.Spec.Params[0].Required, retrieved.Spec.Params[0].Required)

		// Verificar valores de parámetros
		assert.Equal(t, task.Spec.ParamValues["param1"], retrieved.Spec.ParamValues["param1"])
		assert.Equal(t, int32(50), retrieved.Spec.ParamValues["param2"]) // MongoDB convierte a float64 los números
	})

	t.Run("Actualizar Task", func(t *testing.T) {
		// Crear y guardar un Task
		task := createTestTask("MongoDB Task para actualizar")

		err := repo.Save(ctx, task)
		require.NoError(t, err)

		// Modificar y actualizar
		task.Metadata.Description = "Descripción actualizada en MongoDB"
		task.Metadata.Labels = append(task.Metadata.Labels, "updated")
		task.Metadata.Annotations["updated"] = "true"
		task.Spec.Command = []string{"echo", "Updated Command"}
		task.Spec.ParamValues["param1"] = "valor actualizado"

		err = repo.Update(ctx, task)
		require.NoError(t, err)

		// Verificar
		retrieved, err := repo.FindByID(ctx, task.ID)
		require.NoError(t, err)
		assert.Equal(t, "Descripción actualizada en MongoDB", retrieved.Metadata.Description)
		assert.Contains(t, retrieved.Metadata.Labels, "updated")
		assert.Equal(t, "true", retrieved.Metadata.Annotations["updated"])
		assert.Equal(t, []string{"echo", "Updated Command"}, retrieved.Spec.Command)
		assert.Equal(t, "valor actualizado", retrieved.Spec.ParamValues["param1"])
	})

	t.Run("Eliminar Task", func(t *testing.T) {
		task := createTestTask("MongoDB Task para eliminar")

		// Guardar y eliminar
		err := repo.Save(ctx, task)
		require.NoError(t, err)

		err = repo.Delete(ctx, task.ID)
		require.NoError(t, err)

		// Verificar eliminación
		exists, err := repo.Exists(ctx, task.ID)
		require.NoError(t, err)
		assert.False(t, exists)

		// Intentar recuperar debe fallar
		_, err = repo.FindByID(ctx, task.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no encontrada")
	})

	t.Run("FindAll debe retornar todos los Tasks", func(t *testing.T) {
		// Limpiar datos previos
		_, err := db.Collection("tasks").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Crear varios Tasks
		tasks := []*model.Task{
			createTestTask("MongoDB Task 1"),
			createTestTask("MongoDB Task 2"),
			createTestTask("MongoDB Task 3"),
		}

		for _, task := range tasks {
			err := repo.Save(ctx, task)
			require.NoError(t, err)
		}

		// Obtener todas las tareas
		retrieved, err := repo.FindAll(ctx)
		require.NoError(t, err)
		assert.Equal(t, len(tasks), len(retrieved))

		// Verificar que los IDs coincidan (sin importar el orden)
		expectedIDs := make(map[string]bool)
		for _, task := range tasks {
			expectedIDs[task.ID.String()] = true
		}

		retrievedIDs := make(map[string]bool)
		for _, task := range retrieved {
			retrievedIDs[task.ID.String()] = true
		}

		assert.Equal(t, expectedIDs, retrievedIDs)
	})

	t.Run("Búsqueda por criterios múltiples", func(t *testing.T) {
		// Limpiar datos previos
		_, err := db.Collection("tasks").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Crear tareas con diferentes atributos
		taskDev := createTestTask("MongoDB Dev Task")
		taskDev.Metadata.Labels = []string{"dev", "api"}

		taskProd := createTestTask("MongoDB Prod Task")
		taskProd.Metadata.Labels = []string{"prod", "api"}

		taskTest := createTestTask("MongoDB Test UI")
		taskTest.Metadata.Labels = []string{"test", "ui"}
		taskTest.Spec.Command = []string{"npm", "test"}

		tasks := []*model.Task{taskDev, taskProd, taskTest}
		for _, task := range tasks {
			err := repo.Save(ctx, task)
			require.NoError(t, err)
		}

		// Test 1: Buscar por nombre que contiene
		criteria := ports.SearchCriteria{
			Filters: map[string]interface{}{"nameContains": "Prod"},
			Page:    1,
			Size:    10,
		}

		result, err := repo.FindByCriteria(ctx, criteria)
		require.NoError(t, err)
		assert.Equal(t, int64(1), result.TotalElements)
		assert.Equal(t, "MongoDB Prod Task", result.Content[0].Metadata.Name)

		// Test 2: Buscar por etiqueta
		criteria = ports.SearchCriteria{
			Filters: map[string]interface{}{"labels": []string{"api"}},
			Page:    1,
			Size:    10,
		}

		result, err = repo.FindByCriteria(ctx, criteria)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.TotalElements)

		// Test 3: Buscar por comando que contiene
		criteria = ports.SearchCriteria{
			Filters: map[string]interface{}{"commandContains": "npm"},
			Page:    1,
			Size:    10,
		}

		result, err = repo.FindByCriteria(ctx, criteria)
		require.NoError(t, err)
		assert.Equal(t, int64(1), result.TotalElements)
		assert.Equal(t, "MongoDB Test UI", result.Content[0].Metadata.Name)
	})

	t.Run("BatchSave y BatchUpdate", func(t *testing.T) {
		// Limpiar datos previos
		_, err := db.Collection("tasks").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Crear tareas para operaciones por lotes
		tasks := []*model.Task{
			createTestTask("MongoDB Batch Task 1"),
			createTestTask("MongoDB Batch Task 2"),
			createTestTask("MongoDB Batch Task 3"),
		}

		// Guardar por lotes
		err = repo.BatchSave(ctx, tasks)
		require.NoError(t, err)

		// Verificar que se hayan guardado todas
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)

		// Modificar todas las tareas
		for i := range tasks {
			tasks[i].Metadata.Description = "Updated in batch with MongoDB"
			tasks[i].Spec.Command = []string{"echo", "Updated Batch"}
		}

		// Actualizar por lotes
		err = repo.BatchUpdate(ctx, tasks)
		require.NoError(t, err)

		// Verificar las actualizaciones
		for _, task := range tasks {
			retrieved, err := repo.FindByID(ctx, task.ID)
			require.NoError(t, err)
			assert.Equal(t, "Updated in batch with MongoDB", retrieved.Metadata.Description)
			assert.Equal(t, []string{"echo", "Updated Batch"}, retrieved.Spec.Command)
		}
	})

	t.Run("BatchDelete", func(t *testing.T) {
		// Limpiar datos previos
		_, err := db.Collection("tasks").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Crear tareas para eliminar por lotes
		tasks := []*model.Task{
			createTestTask("MongoDB Delete Task 1"),
			createTestTask("MongoDB Delete Task 2"),
		}

		// Guardar las tareas
		for _, task := range tasks {
			err := repo.Save(ctx, task)
			require.NoError(t, err)
		}

		// Verificar que se hayan guardado
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)

		// Crear lista de IDs para eliminar
		var ids []model.AggregateID
		for _, task := range tasks {
			ids = append(ids, task.ID)
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
		_, err := db.Collection("tasks").DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Crear dos tareas en una sola transacción exitosa
		err = repo.WithTransaction(ctx, func(txCtx context.Context) error {
			task1 := createTestTask("MongoDB Tx Task 1")
			task2 := createTestTask("MongoDB Tx Task 2")

			err := repo.Save(txCtx, task1)
			if err != nil {
				return err
			}

			return repo.Save(txCtx, task2)
		})

		require.NoError(t, err)

		// Verificar que ambas se hayan guardado
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)

		// Transacción que debe fallar (rollback automático)
		taskErr := createTestTask("MongoDB Tx Task Error")

		// Guardar primero la tarea para probar el error al guardar duplicado
		err = repo.Save(ctx, taskErr)
		require.NoError(t, err)

		// Intentar una transacción que falla al guardar un duplicado
		err = repo.WithTransaction(ctx, func(txCtx context.Context) error {
			task3 := createTestTask("MongoDB Tx Task 3")

			err := repo.Save(txCtx, task3)
			if err != nil {
				return err
			}

			// Este debería fallar por ID duplicado
			return repo.Save(txCtx, taskErr)
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ya existe una tarea")

		// Verificar que no se haya añadido ninguna tarea nueva (solo debe estar la taskErr original)
		count, err = repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(3), count) // 2 de la transacción exitosa + 1 taskErr
	})
}
