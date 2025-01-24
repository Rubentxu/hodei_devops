// Package main Worker API.
//
// @title Worker API
// @version 1.0
// @description API para gestionar tareas y procesos remotos
// @termsOfService http://swagger.io/terms/
//
// @contact.name API Support
// @contact.email your.email@example.com
//
// @license.name MIT
// @license.url http://opensource.org/licenses/MIT
//
// @host localhost:8080
// @BasePath /
// @schemes http ws
package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"dev.rubentxu.devops-platform/worker/config"
	"dev.rubentxu.devops-platform/worker/internal/adapters/websockets"
	"dev.rubentxu.devops-platform/worker/internal/adapters/worker"
	"dev.rubentxu.devops-platform/worker/internal/adapters/worker/factories"

	_ "dev.rubentxu.devops-platform/worker/docs" // Import generado por swag
	httpSwagger "github.com/swaggo/http-swagger"
)

func main() {
	cfg := config.Load()
	log.Printf("Configuración cargada en worker api: %+v", cfg)
	workerFactory := factories.NewWorkerInstanceFactory(cfg)

	w := worker.NewWorker(
		cfg.WorkerName,
		cfg.MaxConcurrentTasks,
		cfg.StorageType,
		workerFactory,
	)

	wsHandler := websockets.NewWSHandler(w)

	// Obtener el directorio base de la aplicación
	baseDir, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("Error obteniendo directorio base: %v", err)
	}

	// Configurar rutas y servidor HTTP
	mux := setupRoutes(wsHandler, baseDir)
	addr := fmt.Sprintf(":%d", cfg.Port)

	log.Printf("Iniciando servidor en %s", addr)
	log.Printf("Swagger UI disponible en http://localhost%s/swagger/", addr)
	log.Printf("WebSocket disponible en ws://localhost%s/ws", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}

	websockets.HandleSignals()
}

func setupRoutes(wsHandler *websockets.WSHandler, baseDir string) *http.ServeMux {
	mux := http.NewServeMux()

	// WebSocket y Health endpoints
	mux.HandleFunc("/ws", wsHandler.HandleConnection)
	mux.HandleFunc("/health", websockets.HealthHandler)

	// Swagger documentation
	mux.Handle("/swagger/", httpSwagger.WrapHandler)

	return mux
}
