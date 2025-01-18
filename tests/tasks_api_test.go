package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestCreateTask(t *testing.T) {
	// Definimos la estructura que se enviará al WebSocket /tasks/create
	type createTaskRequest struct {
		Name         string            `json:"name"`
		Image        string            `json:"image"`
		Command      []string          `json:"command,omitempty"`
		Env          map[string]string `json:"env,omitempty"`
		WorkingDir   string            `json:"working_dir,omitempty"`
		InstanceType string            `json:"instance_type,omitempty"`
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	taskReq := createTaskRequest{
		Name:         "test-domain-1",
		Image:        "busybox",
		Command:      []string{"echo", "Hola desde createTask test"},
		Env:          map[string]string{"EXAMPLE_KEY": "EXAMPLE_VALUE"},
		WorkingDir:   "/",
		InstanceType: "docker",
	}

	// Construct WebSocket URL (cambia host/puerto si es necesario)
	wsURL := fmt.Sprintf("ws://%s/tasks/create", "localhost:8080")

	// Incluir el token de autorización si es necesario
	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("JWT_TOKEN")))

	// Abrimos la conexión WebSocket
	c, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		t.Fatalf("Error al abrir WebSocket: %v", err)
	}
	defer c.Close()
	defer resp.Body.Close()

	// Enviamos el objeto JSON con la información de la tarea
	if err := c.WriteJSON(taskReq); err != nil {
		t.Fatalf("Error al enviar JSON por WebSocket: %v", err)
	}

	// Leemos mensajes desde el WebSocket hasta que recibamos "done"
	for {
		var msg map[string]interface{}
		if err := c.ReadJSON(&msg); err != nil {
			// Si se cierra la conexión, salimos
			t.Logf("Conexión cerrada: %v", err)
			break
		}
		t.Logf("Mensaje recibido: %+v", msg)

		// Verificamos si es el mensaje final
		if doneVal, ok := msg["done"].(bool); ok && doneVal {
			t.Log("La tarea ha finalizado correctamente.")
			break
		}
	}

	// Agrega las validaciones que necesites, por ejemplo:
	// if msg["message"] != "Process completed successfully" { ... } etc.
}

func TestListTasks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/tasks", apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		t.Fatalf("Error creando request: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("JWT_TOKEN")))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Error ejecutando la petición: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Código de estado inesperado en /tasks: %d (queríamos 200)", resp.StatusCode)
	}

	var tasks []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		t.Errorf("Error al decodificar lista de tareas: %v", err)
		return
	}

	log.Printf("Lista de tareas: %+v", tasks)
	if len(tasks) == 0 {
		t.Log("No hay tareas registradas o la lista está vacía.")
	}
}

func TestCreateAndListTasks(t *testing.T) {
	// Crea una nueva tarea y luego listamos las tareas
	t.Run("CreateTask", TestCreateTask)
	t.Run("ListTasks", TestListTasks)
}
