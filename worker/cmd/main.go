package main

import (
	"fmt"
	"log"
	"net/http"

	"dev.rubentxu.devops-platform/worker/config"
	"dev.rubentxu.devops-platform/worker/internal/adapters/websockets"
	"dev.rubentxu.devops-platform/worker/internal/adapters/worker"
	"dev.rubentxu.devops-platform/worker/internal/adapters/worker/factories"
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

	go func() {
		http.HandleFunc("/ws", wsHandler.HandleConnection)
		setupRoutes(wsHandler)

		addr := fmt.Sprintf(":%d", cfg.Port)
		log.Printf("Iniciando servidor en %s (WebSocket /ws)...", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("Error al iniciar el servidor: %v", err)
		}
	}()

	websockets.HandleSignals()
}

func setupRoutes(wsHandler *websockets.WSHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler.HandleConnection)
	mux.HandleFunc("/health", websockets.HealthHandler)
	return mux
}
