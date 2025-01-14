package main

import (
	"dev.rubentxu.devops-platform/remote_process/internal/adapters/grpc/security"
	"log"
	"time"

	"dev.rubentxu.devops-platform/remote_process/config"
	remote_process_server "dev.rubentxu.devops-platform/remote_process/internal/adapters/grpc"
	"dev.rubentxu.devops-platform/remote_process/internal/application/api"
)

func main() {
	// Crear una instancia de LocalProcessExecutor
	executor := api.NewLocalProcessExecutor()

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
		"/remote_process.RemoteProcessService/StartProcess":  {"admin", "operator"},
		"/remote_process.RemoteProcessService/StopProcess":   {"admin", "operator"},
		"/remote_process.RemoteProcessService/MonitorHealth": {"admin", "operator", "viewer"},
	}

	// Crear interceptor de autenticación
	authInterceptor := security.NewAuthInterceptor(jwtManager, accessibleRoles)

	// Configurar TLS para el servidor
	serverTLSConfig, err := tlsConfig.ConfigureServerTLS()
	if err != nil {
		log.Fatalf("Failed to configure server TLS: %v", err)
	}

	// Iniciar el servidor gRPC con mTLS
	address := ":" + config.GetApplicationPort()
	serverAdapter := remote_process_server.NewAdapter(executor, address)
	serverAdapter.Start(serverTLSConfig, authInterceptor, config.GetEnv())
}
