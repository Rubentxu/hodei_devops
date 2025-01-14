package main

import (
	"log"

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

	// Configurar TLS para el servidor
	serverTLSConfig, err := tlsConfig.ConfigureServerTLS()
	if err != nil {
		log.Fatalf("Failed to configure server TLS: %v", err)
	}

	// Iniciar el servidor gRPC con mTLS
	address := ":" + config.GetApplicationPort()
	serverAdapter := remote_process_server.NewAdapter(executor, address)
	serverAdapter.Start(serverTLSConfig)
}
