package main

import (
	"log"
	"time"

	"dev.rubentxu.devops-platform/remote_process/internal/adapters/grpc/security"

	"dev.rubentxu.devops-platform/remote_process/config"
	remote_process_server "dev.rubentxu.devops-platform/remote_process/internal/adapters/grpc"
	"dev.rubentxu.devops-platform/remote_process/internal/adapters/metrics"
	"dev.rubentxu.devops-platform/remote_process/internal/application/api"
	"dev.rubentxu.devops-platform/remote_process/internal/application/services"
)

func main() {
	log.Println("Iniciando el servicio de proceso remoto...")

	// Crear el recolector de métricas
	metricsCollector := metrics.NewGopsutilCollector()
	log.Println("Recolector de métricas creado.")

	// Crear el servicio de métricas
	metricsService := api.NewMetricsService(metricsCollector)
	log.Println("Servicio de métricas creado.")

	// Crear el ejecutor de procesos
	executor := api.NewLocalProcessExecutor()
	log.Println("Ejecutor de procesos creado.")

	// Crear el servicio de sincronización
	syncService := services.NewSyncService(32 * 1024) // 32KB buffer
	log.Println("Servicio de sincronización creado.")

	// Cargar configuración TLS
	tlsConfig, err := config.LoadTLSConfig()
	if err != nil {
		log.Fatalf("Error al cargar la configuración TLS: %v", err)
	}
	log.Println("Configuración TLS cargada.")

	// Configurar JWT Manager
	jwtManager := security.NewJWTManager(
		config.GetJWTSecret(),
		24*time.Hour,
	)
	log.Println("JWT Manager configurado.")

	// Definir roles y permisos
	accessibleRoles := map[string][]string{
		"/remote_process.RemoteProcessService/StartProcess":      {"admin", "operator"},
		"/remote_process.RemoteProcessService/StopProcess":       {"admin", "operator"},
		"/remote_process.RemoteProcessService/MonitorHealth":     {"admin", "operator", "viewer"},
		"/remote_process.RemoteProcessService/CollectMetrics":    {"admin", "operator", "viewer"},
		"/remote_process.RemoteProcessService/SyncFiles":         {"admin", "operator"},
		"/remote_process.RemoteProcessService/GetRemoteFileList": {"admin", "operator", "viewer"},
		"/remote_process.RemoteProcessService/DeleteRemoteFile":  {"admin", "operator"},
	}
	log.Println("Roles y permisos definidos.")

	// Crear interceptor de autenticación
	authInterceptor := security.NewAuthInterceptor(jwtManager, accessibleRoles)
	log.Println("Interceptor de autenticación creado.")

	// Configurar TLS para el servidor
	serverTLSConfig, err := tlsConfig.ConfigureServerTLS()
	if err != nil {
		log.Fatalf("Error al configurar TLS del servidor: %v", err)
	}
	log.Println("TLS del servidor configurado.")

	// Iniciar el servidor gRPC con todas las dependencias
	address := ":" + config.GetApplicationPort()
	serverAdapter := remote_process_server.NewAdapter(
		executor,
		metricsService,
		syncService,
		address,
	)
	log.Printf("Iniciando servidor gRPC en la dirección %s...", address)
	serverAdapter.Start(serverTLSConfig, authInterceptor, config.GetEnv())
}
