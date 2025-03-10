package main

import (
	"log"

	"dev.rubentxu.devops-platform/orchestrator/config"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/manager"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/resources"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/store"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/utils"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/websockets"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/worker"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/worker/factories"
	"dev.rubentxu.devops-platform/orchestrator/internal/ports"

	"github.com/pocketbase/pocketbase"
)

func main() {
	// Configurar ruta para la base de datos SQLite
	dbPath := "./data/orchestrator.db"

	// Asegurar que el directorio exista
	if err := utils.EnsureDirectoryExists(dbPath); err != nil {
		log.Fatalf("Error al crear directorio para la base de datos: %v", err)
	}

	// Crear nueva instancia de PocketBase
	app := pocketbase.New()
	if app == nil {
		log.Fatalf("Error creando la instancia de PocketBase")
	}

	// Inicializar la base de datos
	if err := store.Initialize(app, dbPath); err != nil {
		log.Fatalf("Error al inicializar la base de datos: %v", err)
	}

	// Cargar configuración
	cfg := config.Load()
	log.Printf("Configuración cargada: %+v", cfg)

	// Inicializar componentes de la aplicación
	workerFactory := factories.NewWorkerInstanceFactory(cfg.GRPC)
	worker := worker.NewWorker(
		cfg.WorkerName,
		cfg.MaxConcurrentTasks,
		workerFactory,
	)

	// Crear stores
	templateStore, err := store.NewPocketBaseStore[ports.WorkerTemplate](app, "worker_templates")
	if err != nil {
		log.Fatalf("Error creando el store para templates: %v", err)
	}

	configStore, err := store.NewPocketBaseStore[map[string]interface{}](app, "resource_pool_configs")
	if err != nil {
		log.Fatalf("Error creando el store para configuraciones: %v", err)
	}

	// Configurar resource pool
	poolFactory := resources.NewDefaultResourcePoolFactory()
	resourcePoolManager, err := resources.NewResourcePoolManager(configStore, templateStore, poolFactory)
	if err != nil {
		log.Fatalf("Error creando el ResourcePoolManager: %v", err)
	}

	// Configurar pool por defecto si está habilitado
	if cfg.DefaultDockerResourcePool {
		if err := setupDefaultPool(resourcePoolManager, poolFactory); err != nil {
			log.Fatalf("Error al configurar pool por defecto: %v", err)
		}
	}

	// Crear manager y websocket handler
	manager, err := manager.New("greedy", cfg.StorageType, worker, 100, app, resourcePoolManager)
	if err != nil {
		log.Fatalf("Error creando el manager: %v", err)
	}

	wsHandler := websockets.NewWSHandler(manager)
	go manager.ProcessTasks()

	// Iniciar servidor
	utils.Start(app, wsHandler, manager, resourcePoolManager)
}

func setupDefaultPool(resourcePoolManager *resources.ResourcePoolManager, poolFactory ports.ResourcePoolFactory) error {
	defaultDockerPoolConfig, err := poolFactory.CreateDefaultResourcePool()
	if err != nil {
		return err
	}
	return resourcePoolManager.SaveConfig(defaultDockerPoolConfig)
}
