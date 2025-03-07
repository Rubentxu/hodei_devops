package main

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository"

	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/worker"

	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/migrations"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/service/resource"
	"log"
	"os"

	adapters "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/resource"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/worker/factories"

	"dev.rubentxu.hodei-devops/hodei-app/config"
	resourcepool "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/resource_pool"
)

func main() {
	connStr := os.Getenv("POSTGRES_CONNECTION_STRING")
	if connStr == "" {
		log.Fatal("POSTGRES_CONNECTION_STRING no está definida en las variables de entorno")
	}

	// Ruta a los scripts de migración
	migrationsPath := "./migrations"

	// Inicializar la base de datos PostgreSQL directamente usando InitializePostgres
	db, err := migrations.InitializePostgres(connStr, migrationsPath)
	if err != nil {
		log.Fatalf("Error al inicializar la base de datos: %s", err)
	}
	defer db.Close()

	// Crear repositorios usando la conexión inicializada
	_ = resourcepool.NewResourcePoolReadRepository(db)
	_ = resourcepool.NewResourcePoolWriteRepository(db)

	// Continuar con la inicialización de la aplicación...

	// Cargar configuración
	cfg := config.Load()
	log.Printf("Configuración cargada: %+v", cfg)

	// Inicializar componentes de la aplicación
	workerFactory := factories.NewWorkerInstanceFactory(cfg.GRPC)
	_ = worker.NewWorker(
		cfg.WorkerName,
		cfg.MaxConcurrentTasks,
		workerFactory,
	)

	// Configurar resource pool
	poolFactory := adapters.NewDefaultResourcePoolFactory()
	resourcePoolManager, err := resource.NewResourcePoolManager(configStore, templateStore, poolFactory)
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
	//manager, err := manager.New("greedy", cfg.StorageType, worker, 100, app, resourcePoolManager)
	//if err != nil {
	//	log.Fatalf("Error creando el manager: %v", err)
	//}
	//
	//wsHandler := websockets.NewWSHandler(manager)
	//go manager.ProcessTasks()
	//
	//// Iniciar servidor
	//utils.Start(app, wsHandler, manager, resourcePoolManager)
}

func setupDefaultPool(resourcePoolManager *resource.ResourcePoolManager, poolFactory ports.ResourcePoolFactory) error {
	defaultDockerPoolConfig, err := poolFactory.CreateDefaultResourcePool()
	if err != nil {
		return err
	}
	return resourcePoolManager.SaveConfig(defaultDockerPoolConfig)
}
