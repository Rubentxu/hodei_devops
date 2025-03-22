package main

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/config"
	router "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/incoming/http"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/worker/factories"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/application/manager"
	"fmt"
	"github.com/rs/zerolog"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Initialize logger
	logFile, err := os.OpenFile("hodei-app.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	// Set global logger to write to file and console
	multi := zerolog.MultiLevelWriter(os.Stdout, logFile)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.SetOutput(multi)

	// Cargar configuración
	cfg := config.Load()
	log.Printf("Configuración cargada: %+v", cfg)

	// Initialize Repositories Container
	repositoriesContainer, err := config.InitializeRepositoriesContainer(cfg)
	if err != nil {
		log.Fatalf("Error initializing repositories container: %v", err)
	}

	log.Println("✅ Conexión a MongoDB establecida correctamente")
	log.Println("Base de datos:", cfg.MongoDBName)
	log.Println("El servidor MongoDB está activo y utilizable")

	// Initialize Services Container
	servicesContainer, err := config.InitializeServicesContainer(repositoriesContainer, cfg)
	if err != nil {
		log.Fatalf("Error initializing services container: %v", err)
		panic(err)
	}

	// Initialize Worker
	taskExecService := servicesContainer.TaskExecutionService
	workerFactory := factories.NewWorkerInstanceFactory(cfg)
	workerInstance := manager.NewWorkerInstanceManager(
		cfg.MaxConcurrentTasks,
		workerFactory,
		&taskExecService,
	)

	// Inicializar HodeiApp
	hodeiApp, err := manager.NewHodeiApp(
		cfg.Scheduler,
		workerInstance,
		servicesContainer.ResourcePoolService,
		servicesContainer.TaskService,
		servicesContainer.WorkerDefinitionService,
		servicesContainer.TaskExecutionService,
		cfg.MaxConcurrentTasks,
		cfg.IdGenerator,
	)
	if err != nil {
		log.Fatalf("Error inicializando HodeiApp: %v", err)
	}

	// Iniciar procesamiento de tareas
	go hodeiApp.ProcessTasks()

	// Configurar el router con todos los servicios necesarios
	router := router.SetupRouter(
		servicesContainer.ResourcePoolService,
		servicesContainer.TaskService,
		servicesContainer.WorkerDefinitionService,
		servicesContainer.TaskExecutionService,
		servicesContainer.AuthService,
		hodeiApp,
	)

	// Inicializar el servidor HTTP
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Iniciar el servidor HTTP en una goroutine
	go func() {
		log.Printf("Servidor HTTP escuchando en el puerto %d", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error iniciando servidor HTTP: %v", err)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create a deadline for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Apagar el servidor gracefully
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Error en el cierre del servidor: %v", err)
	}

	log.Println("Server exiting")
}
