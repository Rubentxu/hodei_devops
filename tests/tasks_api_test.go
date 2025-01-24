package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestCreateTaskViaWS(t *testing.T) {
	type WSMessage struct {
		Action  string      `json:"action"`
		Payload interface{} `json:"payload"`
	}

	type createTaskRequest struct {
		Name         string            `json:"name"`
		Image        string            `json:"image"`
		Command      []string          `json:"command,omitempty"`
		Env          map[string]string `json:"env,omitempty"`
		WorkingDir   string            `json:"working_dir,omitempty"`
		InstanceType string            `json:"instance_type,omitempty"`
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Nueva URL de WebSocket -> /ws
	wsURL := fmt.Sprintf("ws://%s/ws", "localhost:8080")

	// Opcional: header con token
	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("JWT_TOKEN")))

	c, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		t.Fatalf("Error al abrir WebSocket: %v", err)
	}
	defer c.Close()
	defer resp.Body.Close()

	// Preparamos el payload de creación
	taskReq := createTaskRequest{
		Name:         "test-task-WS",
		Image:        "posts_mpv-remote-process",
		Command:      []string{"sh", "-c", "find . -type f -o -type d"},
		Env:          map[string]string{"EXAMPLE_KEY": "example_value"},
		WorkingDir:   "/etc",
		InstanceType: "docker",
	}

	// Enviamos mensaje con "Action" = "create_task"
	msg := WSMessage{
		Action:  "create_task",
		Payload: taskReq,
	}
	if err := c.WriteJSON(msg); err != nil {
		t.Fatalf("Error al enviar JSON por WebSocket: %v", err)
	}

	// Leemos los mensajes devueltos hasta que se cierre la conexión
	for {
		var response WSMessage
		if err := c.ReadJSON(&response); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				t.Log("Conexión cerrada normalmente")
				break
			}
			t.Logf("Error leyendo mensaje: %v", err)
			break
		}
		t.Logf("Mensaje recibido: %+v", response)
	}
}

func TestListTasksViaWS(t *testing.T) {
	type WSMessage struct {
		Action  string      `json:"action"`
		Payload interface{} `json:"payload"`
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wsURL := fmt.Sprintf("ws://%s/ws", "localhost:8080")

	c, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Error abriendo WebSocket para listar tareas: %v", err)
	}
	defer c.Close()
	defer resp.Body.Close()

	// Enviamos mensaje con "Action" = "list_tasks"
	listRequest := WSMessage{Action: "list_tasks"}
	if err := c.WriteJSON(listRequest); err != nil {
		t.Fatalf("Error al enviar list_tasks: %v", err)
	}

	// Leemos los mensajes devueltos hasta que se cierre la conexión
	for {
		var response WSMessage
		if err := c.ReadJSON(&response); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				t.Log("Conexión cerrada normalmente")
				break
			}
			t.Logf("Error leyendo mensaje: %v", err)
			break
		}
		t.Logf("Mensaje recibido: %+v", response)
	}
}
