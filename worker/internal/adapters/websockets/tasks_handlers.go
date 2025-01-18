package websockets

import (
	"context"
	"dev.rubentxu.devops-platform/worker/internal/adapters/worker"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Asumimos que tenemos una instancia global del Worker (o WorkerPort) en main.go
// var GlobalWorker *worker.Worker // Similar a como se mostró antes
var GlobalWorker *worker.Worker

// CreateTaskRequest define los parámetros que el usuario envía para crear una nueva task.
type CreateTaskRequest struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Command      []string          `json:"command,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	InstanceType string            `json:"instance_type,omitempty"`
	Timeout      time.Duration     `json:"timeout,omitempty"`
}

// CreateTaskHandler maneja la creación de una nueva tarea en el Worker.
func CreateTaskHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to websocket: %v", err)
		return
	}
	defer conn.Close()

	var reqBody CreateTaskRequest
	if err := conn.ReadJSON(&reqBody); err != nil {
		_ = sendJSONErrorWebSocket(conn, fmt.Sprintf("Invalid first message: %v", err))
		return
	}

	// Crear la Task (generamos un ID, establecemos estado inicial, etc.)
	newTask := domain.Task{
		ID:   uuid.New(),
		Name: reqBody.Name,
		// Estado inicial: SCHEDULED o similar (depende de tu definición)
		State: domain.Scheduled,
		WorkerInstanceSpec: domain.WorkerInstanceSpec{
			InstanceType:               reqBody.InstanceType,
			RemoteProcessServerAddress: "localhost",
			Image:                      reqBody.Image,
			Command:                    reqBody.Command,
			Env:                        reqBody.Env,
			WorkingDir:                 reqBody.WorkingDir,
		},
	}

	ctx := r.Context()
	if reqBody.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, reqBody.Timeout)
		defer cancel()
	}

	// Insertar la Task en la cola a través del Worker
	outputChan := GlobalWorker.AddTask(newTask)
	defer close(outputChan)

	// 7. Leer del canal y enviar por WebSocket
	for output := range outputChan {
		data := map[string]interface{}{
			"output":   output.Output,
			"is_error": output.IsError,
		}
		if err := conn.WriteJSON(data); err != nil {
			log.Printf("Error sending WebSocket message: %v", err)
			return
		}
	}

	// 8. CUANDO el proceso finaliza, enviar un mensaje final “done”
	doneMsg := map[string]interface{}{
		"done":      true, // Indicador de finalización
		"exit_code": 0,    // Si lo conoces o deseas retornarlo
		"message":   "Process completed successfully",
	}
	if err := conn.WriteJSON(doneMsg); err != nil {
		log.Printf("Error sending done message: %v", err)
	}

	// (Opcional) Enviar un CloseMessage indicando cierre normal
	_ = conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Process finished"),
	)
}

// ListTasksHandler devuelve la lista de tareas registradas en el Worker.
func ListTasksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	tasks, err := GlobalWorker.GetTasks()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al obtener tareas: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		log.Printf("Error codificando la lista de tareas: %v", err)
	}
}
