package main

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

	remote_process_client "dev.rubentxu.devops-platform/adapters/grpc/client/remote_process"
	pb "dev.rubentxu.devops-platform/adapters/grpc/protos/remote_process"
)

// Configuración del cliente
type Config struct {
	RemoteProcessServerAddress string
	Command                    []string
	ProcessID                  string
	CheckInterval              int64
}

// RequestBody define la estructura del JSON que se recibirá
type RequestBody struct {
	RemoteProcessServerAddress string            `json:"remote_process_server_address"`
	Command                    []string          `json:"command,omitempty"`
	ProcessID                  string            `json:"process_id"`
	CheckInterval              int64             `json:"check_interval"`
	Env                        map[string]string `json:"env,omitempty"`
	WorkingDirectory           string            `json:"working_directory,omitempty"`
	Timeout                    time.Duration     `json:"timeout,omitempty"`
}

var client *remote_process_client.Client

func main() {
	http.HandleFunc("/run", runHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/stop", stopHandler)

	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	handleSignals()
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	var reqBody RequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	var err error
	client, err = remote_process_client.New(reqBody.RemoteProcessServerAddress)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, "Failed to create gRPC client")
		return
	}
	defer client.Close()

	// Configurar contexto con timeout si se especifica
	ctx := r.Context()
	if reqBody.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, reqBody.Timeout)
		defer cancel()
	}

	// Configurar headers para SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Crear canal para la salida del proceso
	outputChan := make(chan *pb.ProcessOutput)

	// Iniciar el proceso
	go func() {
		if err := client.StartProcess(ctx, reqBody.ProcessID, reqBody.Command, reqBody.Env, reqBody.WorkingDirectory, outputChan); err != nil {
			log.Printf("Error in StartProcess: %v", err)
		}
	}()

	// Stream de la salida
	for output := range outputChan {
		if _, err := fmt.Fprintf(w, "data: %s\n", output.Output); err != nil {
			log.Printf("Error writing to response: %v", err)
			return
		}
		w.(http.Flusher).Flush()
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	var reqBody RequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	var err error
	client, err = remote_process_client.New(reqBody.RemoteProcessServerAddress)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, "Failed to create gRPC client")
		return
	}
	defer client.Close()

	// Configurar contexto con timeout si se especifica
	ctx := r.Context()
	if reqBody.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, reqBody.Timeout)
		defer cancel()
	}

	// Crear el canal con buffer para evitar bloqueos
	healthChan := make(chan *pb.HealthStatus, 1)

	if err := client.MonitorHealth(ctx, reqBody.ProcessID, reqBody.CheckInterval, healthChan); err != nil {
		sendJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start health monitoring: %v", err))
		return
	}

	// Configurar headers para SSE
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Detectar si el cliente cierra la conexión
	notify := w.(http.CloseNotifier).CloseNotify()

	// Crear un encoder JSON para cada línea
	encoder := json.NewEncoder(w)

	// Stream del estado de salud
	for {
		select {
		case <-notify:
			return
		case healthStatus, ok := <-healthChan:
			if !ok {
				return
			}
			response := HealthResponse{
				ProcessID: healthStatus.ProcessId,
				IsRunning: healthStatus.IsRunning,
				Status:    healthStatus.Status,
				IsHealthy: healthStatus.IsHealthy,
				Timestamp: time.Now().Format(time.RFC3339),
			}

			if err := encoder.Encode(response); err != nil {
				log.Printf("Error encoding health response: %v", err)
				return
			}

			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

			if !healthStatus.IsRunning {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func handleSignals() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		log.Println("Signal received, shutting down...")
		if client != nil {
			client.Close()
		}
		os.Exit(0)
	}
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	var reqBody RequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// Usar timeout del request o valor por defecto
	timeout := reqBody.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var err error
	client, err = remote_process_client.New(reqBody.RemoteProcessServerAddress)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, "Failed to create gRPC client")
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	success, message, err := client.StopProcess(ctx, reqBody.ProcessID)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to stop process: %v", err))
		return
	}

	response := StopResponse{
		Success: success,
		Message: message,
	}
	sendJSONResponse(w, http.StatusOK, response)
}

// Estructuras de respuesta
type HealthResponse struct {
	ProcessID string `json:"process_id"`
	IsRunning bool   `json:"is_running"`
	Status    string `json:"status"`
	IsHealthy bool   `json:"is_healthy"`
	Timestamp string `json:"timestamp"`
}

type StopResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Funciones helper para respuestas JSON
func sendJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func sendJSONError(w http.ResponseWriter, status int, message string) {
	response := StopResponse{
		Success: false,
		Message: message,
	}
	sendJSONResponse(w, status, response)
}
