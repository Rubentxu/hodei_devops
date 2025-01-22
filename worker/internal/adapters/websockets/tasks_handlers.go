package websockets

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dev.rubentxu.devops-platform/worker/internal/adapters/worker"
	"dev.rubentxu.devops-platform/worker/internal/domain"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // En producción, restringir a orígenes válidos
	},
}

type WSHandler struct {
	worker *worker.Worker
}

func NewWSHandler(w *worker.Worker) *WSHandler {
	return &WSHandler{worker: w}
}

type WSMessage struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}

type TaskRequest struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Command      []string          `json:"command,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	InstanceType string            `json:"instance_type,omitempty"`
	Timeout      int               `json:"timeout,omitempty"`
}

type TaskResponse struct {
	TaskID      string `json:"task_id"`
	Status      string `json:"status"`
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
	IsError     bool   `json:"is_error"`
	ExitCode    string `json:"exit_code,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
}

func (h *WSHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer func() {
		// Cierre gradual con timeout extendido
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Closing normally"),
			time.Now().Add(3*time.Second),
		)
		conn.Close()
	}()

	ctx, cancel := context.WithCancelCause(r.Context())
	defer cancel(nil)

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	go h.sendPing(ctx, conn)

	for {
		var msg WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		switch msg.Action {
		case "create_task":
			h.handleCreateTask(ctx, conn, msg.Payload)
		case "stop_task":
			h.handleStopTask(ctx, conn, msg.Payload)
		case "list_tasks":
			h.handleListTasks(ctx, conn)
		default:
			h.sendError(conn, "unknown_action", "Unsupported action type")
		}
	}
}

func (h *WSHandler) handleCreateTask(ctx context.Context, conn *websocket.Conn, payload json.RawMessage) {
	var req TaskRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid_request", "Error decoding task request")
		return
	}

	if req.Image == "" || req.Name == "" {
		h.sendError(conn, "validation_error", "Name and Image are required fields")
		return
	}

	task, taskCtx := h.createTaskFromRequest(ctx, req)
	outputChan, err := h.worker.AddTask(taskCtx, task)
	if err != nil {
		h.sendError(conn, "create_error", fmt.Sprintf("Error creating task: %v", err))
		return
	}

	//defer close(outputChan)

	// 7. Leer del canal y enviar por WebSocket
	for output := range outputChan {
		resp := TaskResponse{
			TaskID:  task.ID.String(),
			Output:  output.Output,
			IsError: output.IsError,
			Status:  "running",
		}
		
		payload, err := json.Marshal(resp)
		if err != nil {
			log.Printf("Error serializando respuesta: %v", err)
			return
		}
		
		msg := WSMessage{
			Action:  "task_output",
			Payload: json.RawMessage(payload),
		}
		
		if err := conn.WriteJSON(msg); err != nil {
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

func (h *WSHandler) createTaskFromRequest(parent context.Context, req TaskRequest) (domain.Task, context.Context) {
	taskCtx, cancel := context.WithCancel(parent)
	if req.Timeout > 0 {
		taskCtx, cancel = context.WithTimeout(taskCtx, time.Duration(req.Timeout)*time.Second)
	}

	go func() {
		<-taskCtx.Done()
		cancel()
	}()

	return domain.Task{
		ID:    uuid.New(),
		Name:  req.Name,
		State: domain.Scheduled,
		WorkerSpec: domain.WorkerSpec{
			Type:       domain.InstanceType(req.InstanceType),
			Image:      req.Image,
			Command:    req.Command,
			Env:        req.Env,
			WorkingDir: req.WorkingDir,
		},
		CreatedAt: time.Now(),
	}, taskCtx
}

func (h *WSHandler) streamTaskOutput(ctx context.Context, conn *websocket.Conn, taskID string, outputChan <-chan *domain.ProcessOutput) {
	defer h.sendTaskCompletion(ctx, conn, taskID)

	for output := range outputChan {
		// Enviar cada output inmediatamente por el WebSocket
		if !h.sendJSON(conn, "task_output", TaskResponse{
			TaskID:  taskID,
			Output:  output.Output,
			IsError: output.IsError,
			Status:  "running",
		}) {
			log.Printf("Error enviando output, cerrando conexión para tarea %s", taskID)
			return
		}

		// Verificar periodicamente si el contexto fue cancelado
		select {
		case <-ctx.Done():
			log.Printf("Contexto cancelado durante streaming de tarea %s", taskID)
			h.worker.StopTask(taskID)
			return
		default:
			// Mantener conexión activa con ping
			conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait))
		}
	}
}

func (h *WSHandler) sendTaskCompletion(ctx context.Context, conn *websocket.Conn, taskID string) {
	task, err := h.worker.GetTask(taskID)
	if err != nil {
		h.sendError(conn, "task_error", fmt.Sprintf("Error getting task status: %v", err))
		return
	}

	resp := TaskResponse{
		TaskID:      taskID,
		Status:      string(task.State),
		CompletedAt: time.Now().Format(time.RFC3339),
	}

	if task.State == domain.Completed {
		resp.ExitCode = "COMPLETED"
	} else {
		resp.ExitCode = "1"
		resp.Error = "Task failed to complete"
	}

	h.sendJSON(conn, "task_completed", resp)
}

func (h *WSHandler) handleStopTask(ctx context.Context, conn *websocket.Conn, payload json.RawMessage) {
	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid_request", "Invalid task ID format")
		return
	}

	if err := h.worker.StopTask(req.TaskID); err != nil {
		h.sendError(conn, "stop_error", err.Error())
		return
	}

	h.sendJSON(conn, "task_stopped", TaskResponse{
		TaskID: req.TaskID,
		Status: "stopped",
	})
}

func (h *WSHandler) handleListTasks(ctx context.Context, conn *websocket.Conn) {
	tasks, err := h.worker.GetTasks()
	if err != nil {
		h.sendError(conn, "list_error", "Error retrieving tasks")
		return
	}

	h.sendJSON(conn, "task_list", tasks)
}

func (h *WSHandler) sendPing(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (h *WSHandler) sendJSON(conn *websocket.Conn, action string, data interface{}) bool {
	raw, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error serializando payload: %v", err)
		return false
	}

	msg := WSMessage{
		Action:  action,
		Payload: json.RawMessage(raw),
	}

	conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("WebSocket write error: %v", err)
		return false
	}
	return true
}

func (h *WSHandler) sendError(conn *websocket.Conn, code string, message string) {
	h.sendJSON(conn, "task_error", TaskResponse{
		IsError:  true,
		ExitCode: code,
		Error:    message,
	})
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status := map[string]interface{}{
		"timestamp": time.Now(),
		"status":    "up",
		"service":   "orchestrator",
	}
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(status)
}

func HandleSignals() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		log.Println("Signal received, shutting down...")
		os.Exit(0)
	}
}
