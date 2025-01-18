package main

import (
	"log"
	"net/http"

	"dev.rubentxu.devops-platform/worker/internal/adapters/websockets"
	"dev.rubentxu.devops-platform/worker/internal/adapters/worker"
)

func main() {
	// Crea e inicializa tu Worker (storage en memoria, por ejemplo).
	websockets.GlobalWorker = worker.New("myWorker", "memory")
	go websockets.GlobalWorker.RunTasks()

	go func() {
		// Endpoints existentes (WebSockets, etc.)
		http.HandleFunc("/run", websockets.RunHandler)
		http.HandleFunc("/health/worker", websockets.WorkerHealthHandler)
		http.HandleFunc("/health", websockets.HealthHandler)
		http.HandleFunc("/stop", websockets.StopHandler)
		http.HandleFunc("/metrics", websockets.MetricsHandler)

		// Nuevos endpoints para gestionar Tasks:
		http.HandleFunc("/tasks/create", websockets.CreateTaskHandler)
		http.HandleFunc("/tasks", websockets.ListTasksHandler)

		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	websockets.HandleSignals()
}
