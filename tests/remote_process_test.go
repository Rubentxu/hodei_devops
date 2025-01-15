package integration

import (
	"bytes"
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

// Variables para URLs base
var (
	// Endpoints HTTP normales (ej: /stop, /health)
	apiBaseURL = "http://localhost:8080"

	// Endpoints WebSocket (ej: /run, /metrics, /health/worker)
	wsBaseURL = "ws://localhost:8080"

	// Dirección del servidor gRPC (interna)
	serverAddr = "localhost:50051"
)

// --------------------------------------------------------------------------
// Estructuras de datos para las pruebas
// --------------------------------------------------------------------------

// ProcessTestCase define la estructura de un caso de prueba
type ProcessTestCase struct {
	Name       string
	ProcessID  string
	Command    []string
	Env        map[string]string
	WorkingDir string
	Duration   time.Duration
	Timeout    time.Duration
}

// RequestBody define la estructura del JSON que se envía
// como primer mensaje WebSocket en /run.
type RequestBody struct {
	RemoteProcessServerAddress string            `json:"remote_process_server_address"`
	Command                    []string          `json:"command"`
	ProcessID                  string            `json:"process_id"`
	CheckInterval              int64             `json:"check_interval"`
	Env                        map[string]string `json:"env,omitempty"`
	WorkingDirectory           string            `json:"working_directory,omitempty"`
	Timeout                    time.Duration     `json:"timeout,omitempty"`
}

// --------------------------------------------------------------------------
// TEST: /run -> Iniciar proceso vía WebSocket
// --------------------------------------------------------------------------
func TestProcessExecution(t *testing.T) {
	for _, tc := range ProcessTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tc.Duration)
			defer cancel()

			processDone := make(chan struct{})
			go runProcess(ctx, tc, processDone, t)

			select {
			case <-processDone:
				t.Logf("Test completed: %s", tc.Name)
			case <-ctx.Done():
				t.Errorf("Test timed out: %s", tc.Name)
			}
		})
	}
}

// runProcess establece una conexión WebSocket con /run, envía el JSON como
// primer mensaje y luego lee la salida en un bucle.
func runProcess(ctx context.Context, tc ProcessTestCase, done chan<- struct{}, t *testing.T) {
	defer close(done)

	// 1. Determinar la URL para el WebSocket (por ejemplo, /run)
	wsURL := fmt.Sprintf("%s/run", wsBaseURL)

	// 2. Crear el Dialer y preparar cabeceras (si tu servidor requiere token, etc.)
	dialer := &websocket.Dialer{}
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("JWT_TOKEN")))

	// 3. Conectarse vía WebSocket
	conn, resp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		t.Errorf("[%s] WebSocket dial error: %v", tc.ProcessID, err)
		return
	}
	defer conn.Close()

	// Revisar el status code de handshake (opcional)
	if resp != nil && resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("[%s] Server did not switch protocols. Got status: %d", tc.ProcessID, resp.StatusCode)
		return
	}

	// 4. Enviar el JSON de configuración como primer mensaje
	reqBody := RequestBody{
		RemoteProcessServerAddress: serverAddr,
		Command:                    tc.Command,
		ProcessID:                  tc.ProcessID,
		CheckInterval:              5,
		Env:                        tc.Env,
		WorkingDirectory:           tc.WorkingDir,
		Timeout:                    tc.Timeout,
	}
	if err := sendJSONWebSocket(conn, reqBody); err != nil {
		t.Errorf("[%s] Error sending JSON config: %v", tc.ProcessID, err)
		return
	}

	// 5. Leer la salida del proceso en un goroutine
	doneReading := make(chan struct{})
	go func() {
		defer close(doneReading)
		for {
			// Salir si el contexto se cerró
			select {
			case <-ctx.Done():
				log.Printf("[%s] Context canceled: %v", tc.ProcessID, ctx.Err())
				return
			default:
			}

			// Leer siguiente mensaje del WebSocket
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				log.Printf("[%s] Error reading WebSocket message: %v", tc.ProcessID, err)
				return
			}
			if msgType == websocket.CloseMessage {
				log.Printf("[%s] Received close message", tc.ProcessID)
				return
			}

			// Interpretar el JSON
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				log.Printf("[%s] Error parsing JSON: %v", tc.ProcessID, err)
				return
			}

			// Si el mensaje contiene "done": true, terminar
			if doneVal, ok := data["done"].(bool); ok && doneVal {
				exitCode := data["exit_code"]
				msgStr := data["message"]
				log.Printf("[%s] Process finished. exit_code=%v, message=%v", tc.ProcessID, exitCode, msgStr)
				return
			}

			// Procesar la salida
			log.Printf("[%s] Process output: %s", tc.ProcessID, string(msg))
		}
	}()

	// Esperar a que termine la lectura o se venza el context
	select {
	case <-ctx.Done():
		log.Printf("[%s] runProcess context done", tc.ProcessID)
		return
	case <-doneReading:
		return
	}
}

// --------------------------------------------------------------------------
// TEST: /stop -> Endpoint HTTP normal para detener el proceso
// --------------------------------------------------------------------------
func TestStopProcess(t *testing.T) {
	// Aquí un ejemplo sencillo. Ajusta según tus necesidades reales.
	processID := "echo-basic"

	reqBody := RequestBody{
		RemoteProcessServerAddress: serverAddr,
		ProcessID:                  processID,
		Timeout:                    10 * time.Second,
	}

	url := fmt.Sprintf("%s/stop", apiBaseURL)
	bodyBuf := createJSONBody(reqBody)
	req, err := http.NewRequest("POST", url, bodyBuf)
	if err != nil {
		t.Fatalf("Error creating stop request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("JWT_TOKEN")))

	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Error executing stop request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Unexpected status code: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	// Podrías leer el body para verificar "success", "message", etc.
	log.Printf("Process %s stopped successfully", processID)
	log.Printf("Response: %v", resp)
}

// ==================================
// Helpers
// ==================================

// sendJSONWebSocket serializa y envía un objeto como mensaje de texto WebSocket
func sendJSONWebSocket(conn *websocket.Conn, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}
	return conn.WriteMessage(websocket.TextMessage, payload)
}

func createJSONBody(reqBody RequestBody) *bytes.Buffer {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Error marshaling request body: %v", err)
		return bytes.NewBuffer(nil)
	}
	return bytes.NewBuffer(jsonData)
}
