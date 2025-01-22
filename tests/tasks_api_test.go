package integration

import (
	"context"
	"encoding/json"
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
		Command:      []string{"echo", "Hola desde WebSocket test"},
		Env:          map[string]string{"EXAMPLE_KEY": "example_value"},
		WorkingDir:   "/",
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

	// Leemos los mensajes devueltos hasta encontrar "task_completed" o error
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

		switch response.Action {
		case "task_created":
			t.Log("Tarea creada correctamente")
		case "task_output":
			t.Log("Recibido output de la tarea")
		case "task_completed":
			t.Log("La tarea ha finalizado correctamente")
			return
		case "task_error":
			t.Errorf("Error en la tarea: %+v", response)
			return
		}
	}
}

func TestListTasksViaWS(t *testing.T) {
	// Ejemplo de test que envía un mensaje "list_tasks" al mismo /ws y
	// lee la lista de tareas en la respuesta.

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

	// Leemos respuesta
	var response WSMessage
	if err := c.ReadJSON(&response); err != nil {
		t.Errorf("Error al leer respuesta de list_tasks: %v", err)
		return
	}
	t.Logf("Respuesta list_tasks: %+v", response)

	if response.Action != "task_list" {
		t.Errorf("Se esperaba respuesta con action=task_list, se obtuvo %v", response.Action)
	}

	// Decodificar la lista de tareas
	// Dependiendo cómo se envíe, puede ser un slice u otro formato
	var tasks []map[string]interface{}
	payloadBytes, _ := json.Marshal(response.Payload)
	if err := json.Unmarshal(payloadBytes, &tasks); err != nil {
		t.Errorf("Error decodificando tareas: %v", err)
	}
	t.Logf("Tareas recibidas: %v", tasks)
}
