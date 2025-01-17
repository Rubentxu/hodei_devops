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

	pb "dev.rubentxu.devops-platform/protos/remote_process"

	"dev.rubentxu.devops-platform/worker/config"
	remote_process_client "dev.rubentxu.devops-platform/worker/internal/adapters/grpc"

	"github.com/gorilla/websocket"
)

// Configuración del cliente
type Config struct {
	RemoteProcessServerAddress string
	Command                    []string
	ProcessID                  string
	CheckInterval              int64
}

// RequestBody define la estructura del JSON que se recibirá
// (se usará en los mensajes WebSocket)
type RequestBody struct {
	RemoteProcessServerAddress string            `json:"remote_process_server_address"`
	Command                    []string          `json:"command,omitempty"`
	ProcessID                  string            `json:"process_id"`
	CheckInterval              int64             `json:"check_interval"`
	Env                        map[string]string `json:"env,omitempty"`
	WorkingDirectory           string            `json:"working_directory,omitempty"`
	Timeout                    time.Duration     `json:"timeout,omitempty"`
}

// (Opcional) Podrías definir structs distintos si cada endpoint WS
// tiene distintos campos. Por ejemplo, para métricas:
type MetricsRequest struct {
	WorkerID    string   `json:"worker_id"`
	Interval    int64    `json:"interval"`
	MetricTypes []string `json:"metric_types"`
}

// Variable global para el cliente gRPC
var client *remote_process_client.Client

// Upgrader de Gorilla para convertir la conexión HTTP a WebSocket
var upgrader = websocket.Upgrader{
	// Permitir todas las conexiones. Ajusta si necesitas validaciones extra.
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	http.HandleFunc("/run", runHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/health/worker", workerHealthHandler)
	http.HandleFunc("/stop", stopHandler)
	http.HandleFunc("/metrics", metricsHandler)

	// Iniciar servidor HTTP en un goroutine
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

	var err error
	client, err = remote_process_client.New(clientConfig)
	if err != nil {
		log.Fatalf("Failed to create gRPC client: %v", err)
	}
	defer client.Close()
}

// ==========================================================================
// 1. /run -> Iniciar un proceso remoto y transmitir su salida vía WebSockets
// ==========================================================================
func runHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Convertir la conexión HTTP en WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to websocket: %v", err)
		return
	}
	defer conn.Close()

	// 2. Leer la configuración (RequestBody) del PRIMER mensaje WebSocket
	var reqBody RequestBody
	if err := conn.ReadJSON(&reqBody); err != nil {
		_ = sendJSONErrorWebSocket(conn, fmt.Sprintf("Invalid first message: %v", err))
		return
	}

	// 3. Crear una nueva configuración del cliente gRPC basado en el request
	clientConfig := &remote_process_client.ClientConfig{
		ServerAddress: reqBody.RemoteProcessServerAddress,
		ClientCert:    config.LoadGRPCConfig().ClientCert,
		ClientKey:     config.LoadGRPCConfig().ClientKey,
		CACert:        config.LoadGRPCConfig().CACert,
		JWTToken:      config.LoadGRPCConfig().JWTToken,
	}
	c, err := remote_process_client.New(clientConfig)
	if err != nil {
		_ = sendJSONErrorWebSocket(conn, fmt.Sprintf("Failed to create gRPC client: %v", err))
		return
	}
	defer c.Close()

	// 4. Configurar contexto con timeout si se especifica
	ctx := r.Context()
	if reqBody.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, reqBody.Timeout)
		defer cancel()
	}

	// 5. Canal para la salida del proceso
	outputChan := make(chan *pb.ProcessOutput)
	defer close(outputChan)

	// 6. Iniciar el proceso en una goroutine
	go func() {
		err := c.StartProcess(ctx, reqBody.ProcessID, reqBody.Command, reqBody.Env, reqBody.WorkingDirectory, outputChan)
		if err != nil {
			log.Printf("Error starting process: %v", err)
			_ = sendJSONErrorWebSocket(conn, fmt.Sprintf("Error starting process: %v", err))
			return
		}
	}()

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

// ====================================================================
// 2. /health/worker -> Monitorizar la salud de un proceso vía WebSockets
// ====================================================================
func workerHealthHandler(w http.ResponseWriter, r *http.Request) {
	// (A) Upgradear conexión a WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to websocket: %v", err)
		return
	}
	defer conn.Close()

	// (B) Leer RequestBody en el primer mensaje WS
	var reqBody RequestBody
	if err := conn.ReadJSON(&reqBody); err != nil {
		_ = sendJSONErrorWebSocket(conn, fmt.Sprintf("Invalid first message: %v", err))
		return
	}

	// (C) Crear cliente gRPC
	clientConfig := &remote_process_client.ClientConfig{
		ServerAddress: reqBody.RemoteProcessServerAddress,
		ClientCert:    config.LoadGRPCConfig().ClientCert,
		ClientKey:     config.LoadGRPCConfig().ClientKey,
		CACert:        config.LoadGRPCConfig().CACert,
		JWTToken:      config.LoadGRPCConfig().JWTToken,
	}
	c, err := remote_process_client.New(clientConfig)
	if err != nil {
		_ = sendJSONErrorWebSocket(conn, fmt.Sprintf("Failed to create gRPC client: %v", err))
		return
	}
	defer c.Close()

	// (D) Contexto con timeout
	ctx := r.Context()
	if reqBody.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, reqBody.Timeout)
		defer cancel()
	}

	// (E) Crear canal
	healthChan := make(chan *pb.HealthStatus, 1)
	defer close(healthChan)

	// (F) Iniciar monitor de salud
	if err := c.MonitorHealth(ctx, reqBody.ProcessID, reqBody.CheckInterval, healthChan); err != nil {
		_ = sendJSONErrorWebSocket(conn, fmt.Sprintf("Failed to start health monitoring: %v", err))
		return
	}

	// (G) Leer actualizaciones y enviarlas por WS
	for healthStatus := range healthChan {
		data := map[string]interface{}{
			"process_id": healthStatus.ProcessId,
			"is_running": healthStatus.IsRunning,
			"status":     healthStatus.Status,
			"is_healthy": healthStatus.IsHealthy,
		}
		if err := conn.WriteJSON(data); err != nil {
			log.Printf("Error sending WebSocket message: %v", err)
			return
		}
	}
}

// ===============================================
// 3. /health -> Health check simple vía HTTP (no WS)
// ===============================================
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status := map[string]interface{}{
		"timestamp": time.Now(),
		"status":    "up",
		"service":   "orchestrator",
	}
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(status)
}

// ====================================================================
// 4. /stop -> Detener un proceso remoto (respuesta simple vía HTTP)
// ====================================================================
func stopHandler(w http.ResponseWriter, r *http.Request) {
	// Se mantiene como HTTP normal
	var reqBody RequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	clientConfig := &remote_process_client.ClientConfig{
		ServerAddress: reqBody.RemoteProcessServerAddress,
		ClientCert:    config.LoadGRPCConfig().ClientCert,
		ClientKey:     config.LoadGRPCConfig().ClientKey,
		CACert:        config.LoadGRPCConfig().CACert,
		JWTToken:      config.LoadGRPCConfig().JWTToken,
	}

	c, err := remote_process_client.New(clientConfig)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create gRPC client: %v", err))
		return
	}
	defer c.Close()

	ctx := r.Context()
	if reqBody.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, reqBody.Timeout)
		defer cancel()
	}

	success, message, err := c.StopProcess(ctx, reqBody.ProcessID)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to stop process: %v", err))
		return
	}

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

// ===========================================================================
// 5. /metrics -> Stream de métricas del worker vía WebSockets (con primer mensaje)
// ===========================================================================
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	// (A) Upgradear a WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to websocket: %v", err)
		return
	}
	defer conn.Close()

	// (B) Leer la config para métricas desde el PRIMER mensaje WS
	var req MetricsRequest
	if err := conn.ReadJSON(&req); err != nil {
		_ = sendJSONErrorWebSocket(conn, fmt.Sprintf("Invalid first message: %v", err))
		return
	}

	// (C) Crear cliente gRPC si no existe
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
			_ = sendJSONErrorWebSocket(conn, fmt.Sprintf("Failed to create gRPC client: %v", err))
			return
		}
	}

	// (D) Si Interval = 0, establecer un default
	interval := req.Interval
	if interval == 0 {
		interval = 1
	}

	// Crear canales
	metricsChan := make(chan *pb.WorkerMetrics)
	errChan := make(chan error, 1)

	// (E) Iniciar streaming de métricas
	go func() {
		defer close(metricsChan)
		defer close(errChan)

		err := client.StreamMetrics(
			r.Context(),
			req.WorkerID,
			req.MetricTypes,
			interval,
			metricsChan,
		)
		if err != nil {
			errChan <- err
			return
		}
	}()

	// (F) Leer en bucle y reenviar al cliente
	for {
		select {
		case e := <-errChan:
			if e != nil {
				_ = sendJSONErrorWebSocket(conn, fmt.Sprintf("Error streaming metrics: %v", e))
			}
			return
		case metrics, ok := <-metricsChan:
			if !ok {
				// Canal cerrado => se terminó el streaming
				return
			}
			if err := conn.WriteJSON(metrics); err != nil {
				log.Printf("Error sending WebSocket message: %v", err)
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

// ============================================================
// Función para manejar señales del sistema (SIGINT, SIGTERM)
// ============================================================
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

// =======================================
// Utilidades para enviar errores en JSON
// =======================================
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

// Versión para WebSocket
func sendJSONErrorWebSocket(conn *websocket.Conn, message string) error {
	data := map[string]interface{}{
		"error": message,
	}
	return conn.WriteJSON(data)
}
