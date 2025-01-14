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
	// Crear el recolector de métricas
	metricsCollector := metrics.NewGopsutilCollector()

	// Crear el servicio de métricas
	metricsService := api.NewMetricsService(metricsCollector)

	// Crear el ejecutor de procesos
	executor := api.NewLocalProcessExecutor()

	// Crear el servicio de sincronización
	syncService := services.NewSyncService(32 * 1024) // 32KB buffer

	// Cargar configuración TLS
	tlsConfig, err := config.LoadTLSConfig()
	if err != nil {
		log.Fatalf("Failed to load TLS config: %v", err)
	}

	// Configurar JWT Manager
	jwtManager := security.NewJWTManager(
		config.GetJWTSecret(),
		24*time.Hour,
	)

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

	// Crear interceptor de autenticación
	authInterceptor := security.NewAuthInterceptor(jwtManager, accessibleRoles)

	// Configurar TLS para el servidor
	serverTLSConfig, err := tlsConfig.ConfigureServerTLS()
	if err != nil {
		log.Fatalf("Failed to configure server TLS: %v", err)
	}

	// Iniciar el servidor gRPC con todas las dependencias
	address := ":" + config.GetApplicationPort()
	serverAdapter := remote_process_server.NewAdapter(
		executor,
		metricsService,
		syncService,
		address,
	)
	serverAdapter.Start(serverTLSConfig, authInterceptor, config.GetEnv())
}
