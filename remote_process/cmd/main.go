package main

import (
	remote_process_server "dev.rubentxu.devops-platform/remote_process/internal/adapters/grpc"
	"dev.rubentxu.devops-platform/remote_process/internal/application/api"
	"log"
)

func main() {
	// Crear una instancia de LocalProcessExecutor
	executor := api.NewLocalProcessExecutor()

	// Iniciar el servidor gRPC en un puerto específico
	address := ":50051"
	if err := remote_process_server.Start(address, executor); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
