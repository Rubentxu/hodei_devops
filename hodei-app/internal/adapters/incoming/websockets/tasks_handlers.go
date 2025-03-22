package websockets

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator"
	"log"
	"net/http"
	"time"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 30 * time.Second
	pongWait       = 120 * time.Second
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
	HodeiApp  ports.HodeiAppManager
	validator *validator.Validate
}

func NewWSHandler(hodeiApp ports.HodeiAppManager, validator *validator.Validate) *WSHandler {
	return &WSHandler{
		HodeiApp:  hodeiApp,
		validator: validator,
	}
}

// @title Worker WebSocket API
// @version 1.0
// @description API WebSocket para gestionar tareas
// @BasePath /

// HandleConnection godoc
// @Summary Gestiona conexiones WebSocket para tareas
// @Description ConnectionInfo WebSocket para gestionar tareas en tiempo real. Soporta las siguientes acciones:
// @Description - create_task: Crear una nueva tarea
// @Description - stop_task: Detener una tarea en ejecución
// @Description - list_tasks: Listar todas las tareas
// @Tags WebSocket
// @Accept json
// @Produce json
// @Param client_id query string false "ID del cliente para tracking"
// @Success 101 {string} string "Switching Protocols"
// @Failure 400 {object} ErrorResponse
// @Router /ws [get]
func (h *WSHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	ctx, cancel := context.WithCancelCause(r.Context())
	defer cancel(nil)

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Enviar mensaje de bienvenida
	h.sendJSON(conn, "connection_established", map[string]string{"message": "Connection established"})

	go h.sendPing(ctx, conn)

	for {
		var msg WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		// Enviar notificación de procesamiento
		h.sendJSON(conn, "processing_request", map[string]string{"action": msg.Action, "payload": string(msg.Payload)})

		switch msg.Action {
		case "execute_task":
			h.handleExecuteTask(ctx, conn, msg.Payload)
		case "stop_task":
			h.handleStopTask(ctx, conn, msg.Payload)
		default:
			h.sendError(conn, "unknown_action", "Unsupported action type")
		}
	}
}

func (h *WSHandler) handleExecuteTask(ctx context.Context, conn *websocket.Conn, payload json.RawMessage) {
	var taskExecRequest model.TaskExecutionRequest
	if err := json.Unmarshal(payload, &taskExecRequest); err != nil {
		h.sendError(conn, "invalid_request", "Error decoding task request")
		return
	}

	// Validate required fields
	if taskExecRequest.TaskID == "" {
		h.sendError(conn, "validation_error", "Task ID is required")
		return
	}

	log.Printf("Creating task with ID %s", taskExecRequest.TaskID)
	err := h.validator.Struct(taskExecRequest)
	if err != nil {
		h.sendError(conn, "validation_error", fmt.Sprintf("Validation error: %v", err))
		return
	}
	// Call HodeiApp to execute the task
	taskContextResult, err := h.HodeiApp.AddTask(taskExecRequest, ctx)
	if err != nil {
		h.sendError(conn, "task_creation_error", fmt.Sprintf("Error creating task: %v", err))
		return
	}

	// Send task creation confirmation
	h.sendJSON(conn, "task_created", map[string]string{
		"task_id": taskExecRequest.TaskID.String(),
		"status":  "running",
	})

	// Process task output in a goroutine
	go h.processTaskOutput(conn, taskContextResult, taskExecRequest.TaskID)
}

func (h *WSHandler) processTaskOutput(conn *websocket.Conn, taskContext ports.TaskContext, taskID model.AggregateID) {
	// Leer del canal de salida y enviar por WebSocket
	for {
		select {
		case output, ok := <-taskContext.OutputChan:
			if !ok {
				// Canal cerrado, la tarea ha terminado
				h.sendJSON(conn, "task_completed", map[string]string{
					"task_id": taskID.String(),
					"status":  "completed",
				})
				return
			}

			// Enviar output al cliente
			resp := TaskResponse{
				TaskID:  taskID.String(),
				Output:  output.Output,
				IsError: output.IsError,
				Status:  output.Status.String(),
			}
			h.sendJSON(conn, "task_output", resp)

		case <-taskContext.Ctx.Done():
			// Contexto cancelado, la tarea ha sido detenida
			h.sendJSON(conn, "task_stopped", map[string]string{
				"task_id": taskID.String(),
				"status":  "stopped",
			})
			return
		}
	}
}

type WSMessage struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}

type TaskResponse struct {
	TaskID  string `json:"task_id"`
	Output  string `json:"output"`
	IsError bool   `json:"is_error"`
	Status  string `json:"status"`
}

func (h *WSHandler) sendJSON(conn *websocket.Conn, action string, payload interface{}) {
	message, err := json.Marshal(map[string]interface{}{
		"action":  action,
		"payload": payload,
	})
	if err != nil {
		log.Printf("Error serializando mensaje: %v", err)
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
		log.Printf("Error enviando mensaje: %v", err)
	}
}

func (h *WSHandler) sendPing(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait))
			if err != nil {
				log.Println("Ping:", err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (h *WSHandler) handleStopTask(ctx context.Context, conn *websocket.Conn, payload json.RawMessage) {
	var req TaskStopRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid_request", "Error decoding stop task request")
		return
	}

	// Validar ID de tarea
	if req.TaskID == "" {
		h.sendError(conn, "validation_error", "Task ID is required")
		return
	}

	// Crear el contexto y canales necesarios para la tarea
	taskID := model.AggregateID(req.TaskID) // Convertir el string ID a AggregateID

	outputChan := make(chan model.ProcessOutput)
	stateChan := make(chan model.TaskState)
	errChan := make(chan error)
	ctxWithCancel, cancel := context.WithCancel(ctx)

	// Crear el TaskContext para la detención
	taskCtx := ports.TaskContext{
		Execution: model.TaskExecution{
			ID: taskID,
		},
		OutputChan: outputChan,
		StateChan:  stateChan,
		ErrChan:    errChan,
		Ctx:        ctxWithCancel,
	}

	// Llamar a HodeiApp para detener la tarea
	if err := h.HodeiApp.StopTask(taskCtx); err != nil {
		h.sendError(conn, "stop_error", fmt.Sprintf("Error stopping task: %v", err))
		cancel() // Cancelar el contexto
		close(outputChan)
		return
	}

	// Enviar confirmación de detención
	h.sendJSON(conn, "task_stopped", map[string]string{
		"task_id": req.TaskID,
		"status":  "stopped",
	})

	// Limpiar recursos
	cancel()
	close(outputChan)
}

type TaskStopRequest struct {
	TaskID string `json:"task_id"`
}

func (h *WSHandler) handleListTasks(ctx context.Context, conn *websocket.Conn) {
	//TODO implementar
	h.sendError(conn, "not_implemented", "Not implemented")
}

func (h *WSHandler) closeConnection(conn *websocket.Conn, closeCode int, message string) {
	err := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, message), time.Now().Add(writeWait))
	if err != nil {
		log.Printf("Error al enviar mensaje de cierre: %v", err)
		conn.Close()
		return
	}

	time.Sleep(time.Second) // Esperar a que el cliente reciba el mensaje
	conn.Close()
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h *WSHandler) sendError(conn *websocket.Conn, code string, message string) {
	h.sendJSON(conn, "task_error", ErrorResponse{
		Code:    code,
		Message: message,
	})
}
