package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	pb "dev.rubentxu.devops-platform/protos/remote_process"

	"dev.rubentxu.devops-platform/orchestrator/config"
	remote_process_client "dev.rubentxu.devops-platform/orchestrator/internal/adapters/grpc"
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
	http.HandleFunc("/health/worker", workerHealthHandler)
	http.HandleFunc("/stop", stopHandler)
	http.HandleFunc("/metrics", metricsHandler)

	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	handleSignals()

	// Cargar configuración gRPC
	grpcConfig := config.LoadGRPCConfig()

	// Crear cliente gRPC con autenticación
	clientConfig := &remote_process_client.ClientConfig{
		ServerAddress: grpcConfig.ServerAddress,
		ClientCert:    grpcConfig.ClientCert,
		ClientKey:     grpcConfig.ClientKey,
		CACert:        grpcConfig.CACert,
		JWTToken:      grpcConfig.JWTToken,
	}

	client, err := remote_process_client.New(clientConfig)
	if err != nil {
		log.Fatalf("Failed to create gRPC client: %v", err)
	}
	defer client.Close()
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	var reqBody RequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// Crear configuración del cliente con los valores por defecto de TLS
	clientConfig := &remote_process_client.ClientConfig{
		ServerAddress: reqBody.RemoteProcessServerAddress,
		ClientCert:    config.LoadGRPCConfig().ClientCert, // Usar certificados configurados
		ClientKey:     config.LoadGRPCConfig().ClientKey,
		CACert:        config.LoadGRPCConfig().CACert,
		JWTToken:      config.LoadGRPCConfig().JWTToken,
	}

	var err error
	client, err = remote_process_client.New(clientConfig)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create gRPC client: %v", err))
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
	defer close(outputChan)

	// Iniciar el proceso en una goroutine
	go func() {
		err := client.StartProcess(ctx, reqBody.ProcessID, reqBody.Command, reqBody.Env, reqBody.WorkingDirectory, outputChan)
		if err != nil {
			log.Printf("Error starting process: %v", err)
			// Enviar error al cliente a través de SSE
			event := fmt.Sprintf("data: {\"error\": \"%v\"}\n\n", err)
			if _, err := fmt.Fprint(w, event); err != nil {
				log.Printf("Error sending error event: %v", err)
			}
			w.(http.Flusher).Flush()
			return
		}
	}()

	// Enviar la salida del proceso al cliente a través de SSE
	for output := range outputChan {
		event := fmt.Sprintf("data: {\"output\": \"%s\", \"is_error\": %v}\n\n", output.Output, output.IsError)
		if _, err := fmt.Fprint(w, event); err != nil {
			log.Printf("Error sending event: %v", err)
			return
		}
		w.(http.Flusher).Flush()
	}
}

func workerHealthHandler(w http.ResponseWriter, r *http.Request) {
	var reqBody RequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// Crear configuración del cliente con los valores por defecto de TLS
	clientConfig := &remote_process_client.ClientConfig{
		ServerAddress: reqBody.RemoteProcessServerAddress,
		ClientCert:    config.LoadGRPCConfig().ClientCert,
		ClientKey:     config.LoadGRPCConfig().ClientKey,
		CACert:        config.LoadGRPCConfig().CACert,
		JWTToken:      config.LoadGRPCConfig().JWTToken,
	}

	var err error
	client, err = remote_process_client.New(clientConfig)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create gRPC client: %v", err))
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

	// Crear el canal con buffer para evitar bloqueos
	healthChan := make(chan *pb.HealthStatus, 1)
	defer close(healthChan)

	if err := client.MonitorHealth(ctx, reqBody.ProcessID, reqBody.CheckInterval, healthChan); err != nil {
		sendJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start health monitoring: %v", err))
		return
	}

	// Enviar actualizaciones de estado al cliente a través de SSE
	for healthStatus := range healthChan {
		event := fmt.Sprintf("data: {\"process_id\": \"%s\", \"is_running\": %v, \"status\": \"%s\", \"is_healthy\": %v}\n\n",
			healthStatus.ProcessId,
			healthStatus.IsRunning,
			healthStatus.Status,
			healthStatus.IsHealthy)

		if _, err := fmt.Fprint(w, event); err != nil {
			log.Printf("Error sending health event: %v", err)
			return
		}
		w.(http.Flusher).Flush()
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Determinar si es un health check del worker o de la aplicación

	status := map[string]interface{}{
		"timestamp": time.Now(),
	}

	// Health check de la aplicación
	status["status"] = "up"
	status["service"] = "orchestrator"
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(status)
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

	// Crear configuración del cliente con los valores por defecto de TLS
	clientConfig := &remote_process_client.ClientConfig{
		ServerAddress: reqBody.RemoteProcessServerAddress,
		ClientCert:    config.LoadGRPCConfig().ClientCert,
		ClientKey:     config.LoadGRPCConfig().ClientKey,
		CACert:        config.LoadGRPCConfig().CACert,
		JWTToken:      config.LoadGRPCConfig().JWTToken,
	}

	var err error
	client, err = remote_process_client.New(clientConfig)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create gRPC client: %v", err))
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

	success, message, err := client.StopProcess(ctx, reqBody.ProcessID)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to stop process: %v", err))
		return
	}

	// Enviar respuesta JSON
	w.Header().Set("Content-Type", "application/json")
	response := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: success,
		Message: message,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	// Configurar headers para SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Obtener parámetros
	workerID := r.URL.Query().Get("worker_id")
	intervalStr := r.URL.Query().Get("interval")
	metricTypes := r.URL.Query()["metric_types"]

	interval, err := strconv.ParseInt(intervalStr, 10, 64)
	if err != nil {
		interval = 1 // valor por defecto
	}

	// Crear cliente gRPC si no existe
	if client == nil {
		clientConfig := &remote_process_client.ClientConfig{
			ServerAddress: config.LoadGRPCConfig().ServerAddress,
			ClientCert:    config.LoadGRPCConfig().ClientCert,
			ClientKey:     config.LoadGRPCConfig().ClientKey,
			CACert:        config.LoadGRPCConfig().CACert,
			JWTToken:      config.LoadGRPCConfig().JWTToken,
		}

		var err error
		client, err = remote_process_client.New(clientConfig)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create gRPC client: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Crear canal para métricas
	metricsChan := make(chan *pb.WorkerMetrics)
	errChan := make(chan error, 1)

	// Iniciar streaming de métricas
	go func() {
		defer close(metricsChan)
		defer close(errChan)

		err := client.StreamMetrics(r.Context(), workerID, metricTypes, interval, metricsChan)
		if err != nil {
			errChan <- err
			return
		}
	}()

	// Flusher para SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Enviar métricas como eventos SSE
	for {
		select {
		case err := <-errChan:
			fmt.Fprintf(w, "data: {\"error\": \"%v\"}\n\n", err)
			flusher.Flush()
			return
		case metrics, ok := <-metricsChan:
			if !ok {
				return
			}
			// Convertir métricas a JSON
			data, err := json.Marshal(metrics)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// Función auxiliar para enviar errores JSON
func sendJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	response := struct {
		Error string `json:"error"`
	}{
		Error: message,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding error response: %v", err)
	}
}
