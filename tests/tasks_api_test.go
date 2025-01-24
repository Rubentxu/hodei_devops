package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// WSMessage define la estructura de los mensajes WebSocket
type WSMessage struct {
	Action  string      `json:"action"`
	Payload interface{} `json:"payload"`
}

// createTaskRequest define la estructura para crear una tarea
type createTaskRequest struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Command      []string          `json:"command,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	InstanceType string            `json:"instance_type,omitempty"`
}

// TaskTestCase define la estructura de un caso de prueba de tarea
type TaskTestCase struct {
	Name         string
	Image        string
	Command      []string
	Env          map[string]string
	WorkingDir   string
	InstanceType string
	Duration     time.Duration
}

func TestCreateTaskViaWS(t *testing.T) {
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

var TaskTestCases = []TaskTestCase{
	{
		Name:         "list-files-task",
		Image:        "posts_mpv-remote-process",
		Command:      []string{"sh", "-c", "find /etc -type f | head -n 5"},
		Env:          map[string]string{"SEARCH_PATH": "/etc"},
		WorkingDir:   "/",
		InstanceType: "docker",
		Duration:     10 * time.Second,
	},
	{
		Name:         "system-info-task",
		Image:        "posts_mpv-remote-process",
		Command:      []string{"sh", "-c", "uname -a && cat /etc/os-release"},
		Env:          map[string]string{},
		WorkingDir:   "/",
		InstanceType: "docker",
		Duration:     10 * time.Second,
	},
	{
		Name:         "memory-info-task",
		Image:        "posts_mpv-remote-process",
		Command:      []string{"sh", "-c", "free -h && df -h"},
		Env:          map[string]string{},
		WorkingDir:   "/",
		InstanceType: "docker",
		Duration:     10 * time.Second,
	},
	{
		Name:         "network-test-task",
		Image:        "posts_mpv-remote-process",
		Command:      []string{"sh", "-c", "ip addr && netstat -tulpn"},
		Env:          map[string]string{},
		WorkingDir:   "/",
		InstanceType: "docker",
		Duration:     10 * time.Second,
	},
	{
		Name:         "process-list-task",
		Image:        "posts_mpv-remote-process",
		Command:      []string{"sh", "-c", "ps aux | head -n 5"},
		Env:          map[string]string{},
		WorkingDir:   "/",
		InstanceType: "docker",
		Duration:     10 * time.Second,
	},
	{
		Name:         "custom-env-task",
		Image:        "posts_mpv-remote-process",
		Command:      []string{"sh", "-c", "env | sort"},
		Env:          map[string]string{"CUSTOM_VAR1": "value1", "CUSTOM_VAR2": "value2"},
		WorkingDir:   "/",
		InstanceType: "docker",
		Duration:     10 * time.Second,
	},
}

func TestParallelTasks(t *testing.T) {
	// Número máximo de tareas concurrentes
	maxConcurrent := 3

	// Crear un canal para limitar la concurrencia
	semaphore := make(chan struct{}, maxConcurrent)

	// Crear un WaitGroup para esperar a que todas las tareas terminen
	var wg sync.WaitGroup

	for _, tc := range TaskTestCases {
		tc := tc // Crear una nueva variable para cada iteración
		wg.Add(1)

		go func() {
			// Adquirir un slot del semáforo
			semaphore <- struct{}{}
			defer func() {
				// Liberar el slot del semáforo
				<-semaphore
				wg.Done()
			}()

			t.Run(tc.Name, func(t *testing.T) {
				t.Parallel() // Marcar el subtest como paralelo

				// Contexto con timeout de 30 segundos
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()

				// Establecer conexión WebSocket
				wsURL := fmt.Sprintf("ws://%s/ws", "localhost:8080")
				header := http.Header{}
				header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("JWT_TOKEN")))

				c, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
				if err != nil {
					t.Fatalf("[%s] Error al abrir WebSocket: %v", tc.Name, err)
				}

				// No cerramos la conexión aquí, esperamos a que el servidor la cierre
				defer resp.Body.Close()

				// Crear y enviar la tarea
				taskReq := createTaskRequest{
					Name:         tc.Name,
					Image:        tc.Image,
					Command:      tc.Command,
					Env:          tc.Env,
					WorkingDir:   tc.WorkingDir,
					InstanceType: tc.InstanceType,
				}

				msg := WSMessage{
					Action:  "create_task",
					Payload: taskReq,
				}

				if err := c.WriteJSON(msg); err != nil {
					t.Fatalf("[%s] Error al enviar JSON por WebSocket: %v", tc.Name, err)
				}

				// Canal para señalizar que la tarea ha terminado
				taskFinished := make(chan struct{})
				taskError := make(chan error, 1)

				// Leer respuestas
				go func() {
					defer close(taskFinished)
					for {
						var response WSMessage
						err := c.ReadJSON(&response)
						if err != nil {
							if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
								t.Logf("[%s] Conexión cerrada por el servidor", tc.Name)
								return
							}
							select {
							case taskError <- fmt.Errorf("[%s] Error leyendo mensaje: %v", tc.Name, err):
							default:
							}
							return
						}

						t.Logf("[%s] Mensaje recibido: %+v", tc.Name, response)

						// Verificar si la tarea ha terminado
						if payload, ok := response.Payload.(map[string]interface{}); ok {
							if status, ok := payload["status"].(string); ok {
								if status == "FINISHED" {
									t.Logf("[%s] Tarea completada exitosamente", tc.Name)
									// No cerramos la conexión, esperamos a que el servidor lo haga
									return
								}
							}
						}
					}
				}()

				// Esperar a que termine la tarea o se agote el tiempo
				select {
				case <-ctx.Done():
					if ctx.Err() == context.DeadlineExceeded {
						t.Errorf("[%s] Timeout alcanzado después de 90 segundos", tc.Name)
					}
				case err := <-taskError:
					t.Error(err)
				case <-taskFinished:
					t.Logf("[%s] Tarea completada correctamente", tc.Name)
				}
			})
		}()
	}

	// Esperar a que todas las tareas terminen
	wg.Wait()
}
