package main

import (
	"log"

	"dev.rubentxu.devops-platform/remote_process/config"
	remote_process_server "dev.rubentxu.devops-platform/remote_process/internal/adapters/grpc"
	"dev.rubentxu.devops-platform/remote_process/internal/application/api"
	"google.golang.org/grpc/credentials"
)

func main() {
	// Crear una instancia de LocalProcessExecutor
	executor := api.NewLocalProcessExecutor()

	// Obtener los valores de las variables de entorno
	certFile := config.GetCertFile()
	keyFile := config.GetKeyFile()
	applicationPort := config.GetApplicationPort()

	// Verificar que las variables de entorno no estén vacías
	if certFile == "" || keyFile == "" {
		log.Fatal("CERT_FILE and KEY_FILE environment variables must be set")
	}

	creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		log.Fatal("Failed to load TLS credentials", err)
	}

	// Iniciar el servidor gRPC en un puerto específico
	address := ":" + applicationPort
	serverAdapter := remote_process_server.NewAdapter(executor, address)
	serverAdapter.Start(creds)
}
