package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestEnvironment struct {
	ServerContainer testcontainers.Container
	ClientContainer testcontainers.Container
	NetworkName     string
	Context         context.Context
}

func setupTestEnvironment(t *testing.T) (*TestEnvironment, error) {
	ctx := context.Background()

	// Crear red de Docker
	networkName := fmt.Sprintf("test-network-%s", uuid.New().String())
	network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name: networkName,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %v", err)
	}

	// Configurar variables de entorno comunes
	commonEnv := map[string]string{
		"CA_CERT_PATH": "/certs/ca-cert.pem",
		"ENV":          "development",
	}

	// Configurar el contenedor del servidor
	serverEnv := map[string]string{
		"SERVER_CERT_PATH": "/certs/server-cert.pem",
		"SERVER_KEY_PATH":  "/certs/server-key.pem",
		"APPLICATION_PORT": "50051",
		"JWT_SECRET":       os.Getenv("JWT_SECRET"),
	}
	for k, v := range commonEnv {
		serverEnv[k] = v
	}

	serverReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../../",
			Dockerfile: "build/server/Dockerfile",
		},
		Networks:     []string{networkName},
		Name:         fmt.Sprintf("server-%s", uuid.New().String()),
		ExposedPorts: []string{"50051:50051"},
		Env:          serverEnv,
		WaitingFor:   wait.ForLog("Server started").WithStartupTimeout(60 * time.Second),
	}

	// Configurar el contenedor del cliente
	clientEnv := map[string]string{
		"CLIENT_CERT_PATH":    "/certs/client-cert.pem",
		"CLIENT_KEY_PATH":     "/certs/client-key.pem",
		"GRPC_SERVER_ADDRESS": "server:50051",
		"JWT_TOKEN":           os.Getenv("JWT_TOKEN"),
	}
	for k, v := range commonEnv {
		clientEnv[k] = v
	}

	clientReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../../",
			Dockerfile: "build/client/Dockerfile",
		},
		Networks:     []string{networkName},
		Name:         fmt.Sprintf("client-%s", uuid.New().String()),
		ExposedPorts: []string{"8080:8080"},
		Env:          clientEnv,
		WaitingFor:   wait.ForHTTP("/health").WithPort("8080/tcp"),
	}

	// Mejorar el manejo de errores y limpieza
	env := &TestEnvironment{
		NetworkName: networkName,
		Context:     ctx,
	}

	// Asegurar limpieza en caso de error
	defer func() {
		if err != nil {
			env.Cleanup()
		}
	}()

	// Configurar el contenedor del servidor con mejor logging
	serverC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: serverReq,
		Started:          true,
		Logger:           testcontainers.TestLogger(t),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start server container: %v", err)
	}
	env.ServerContainer = serverC

	// Esperar a que el servidor esté realmente listo
	time.Sleep(5 * time.Second)

	// Configurar el contenedor del cliente con mejor logging
	clientC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: clientReq,
		Started:          true,
		Logger:           testcontainers.TestLogger(t),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start client container: %v", err)
	}
	env.ClientContainer = clientC

	return env, nil
}

func (env *TestEnvironment) Cleanup() {
	ctx := context.Background()

	if env.ClientContainer != nil {
		if err := env.ClientContainer.Terminate(ctx); err != nil {
			fmt.Printf("Error terminating client container: %v\n", err)
		}
	}

	if env.ServerContainer != nil {
		if err := env.ServerContainer.Terminate(ctx); err != nil {
			fmt.Printf("Error terminating server container: %v\n", err)
		}
	}

	// Limpiar la red
	if env.NetworkName != "" {
		if err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
			NetworkRequest: testcontainers.NetworkRequest{
				Name:   env.NetworkName,
				Action: testcontainers.NetworkRemove,
			},
		}); err != nil {
			fmt.Printf("Error removing network %s: %v\n", env.NetworkName, err)
		}
	}
}
