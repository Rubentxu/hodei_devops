package main

import (
	remote_process_server "dev.rubentxu.devops-platform/adapters/grpc/server/remote_process"
	"dev.rubentxu.devops-platform/adapters/local"
	"log"
)

func main() {
	// Crear una instancia de LocalProcessExecutor
	executor := local.NewLocalProcessExecutor()

	// Iniciar el servidor gRPC en un puerto específico
	address := ":50051"
	if err := remote_process_server.Start(address, executor); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
