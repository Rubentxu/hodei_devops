package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
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
	Command                    []string          `json:"command"`
	ProcessID                  string            `json:"process_id"`
	CheckInterval              int64             `json:"check_interval"`
	Env                        map[string]string `json:"env"`
	WorkingDirectory           string            `json:"working_directory"`
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
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	config := Config{
		RemoteProcessServerAddress: reqBody.RemoteProcessServerAddress,
		Command:                    reqBody.Command,
		ProcessID:                  reqBody.ProcessID,
		CheckInterval:              reqBody.CheckInterval,
	}

	var err error
	client, err = remote_process_client.New(config.RemoteProcessServerAddress)
	if err != nil {
		http.Error(w, "Failed to create gRPC client", http.StatusInternalServerError)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)
	go runProcess(ctx, client, config, reqBody.Env, reqBody.WorkingDirectory, w, &wg)

	wg.Wait()
}

func runProcess(ctx context.Context, client *remote_process_client.Client, config Config, env map[string]string, workingDir string, w http.ResponseWriter, wg *sync.WaitGroup) {
	defer wg.Done()

	// Create a channel to capture the output
	outputChan := make(chan *pb.ProcessOutput)

	// Start the process and redirect its output to the channel
	go func() {
		if err := client.StartProcess(ctx, config.ProcessID, config.Command, env, workingDir, outputChan); err != nil {
			log.Printf("Error in StartProcess: %v", err)
		}
	}()

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Stream the output to the HTTP response
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
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	config := Config{
		RemoteProcessServerAddress: reqBody.RemoteProcessServerAddress,
		ProcessID:                  reqBody.ProcessID,
		CheckInterval:              reqBody.CheckInterval,
	}

	var err error
	client, err = remote_process_client.New(config.RemoteProcessServerAddress)
	if err != nil {
		http.Error(w, "Failed to create gRPC client", http.StatusInternalServerError)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Crear el canal con buffer para evitar bloqueos
	healthChan := make(chan *pb.HealthStatus, 1)

	if err := client.MonitorHealth(ctx, config.ProcessID, config.CheckInterval, healthChan); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start health monitoring: %v", err), http.StatusInternalServerError)
		return
	}

	// Configurar headers para SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Detectar si el cliente cierra la conexión
	notify := w.(http.CloseNotifier).CloseNotify()

	// Stream del estado de salud a la respuesta HTTP
	for {
		select {
		case <-notify:
			// Cliente cerró la conexión
			if _, err := fmt.Fprintf(w, "healthCheck status: Connection closed by client\n"); err != nil {
				log.Printf("Error writing final message: %v", err)
			}
			w.(http.Flusher).Flush()
			cancel()
			return
		case healthStatus, ok := <-healthChan:
			if !ok {
				if _, err := fmt.Fprintf(w, "healthCheck status: Monitoring completed successfully\n"); err != nil {
					log.Printf("Error writing final message: %v", err)
				}
				w.(http.Flusher).Flush()
				return
			}
			if _, err := fmt.Fprintf(w, "healthCheck status: %s, isRunning: %t\n",
				healthStatus.Status, healthStatus.IsRunning); err != nil {
				log.Printf("Error writing to response: %v", err)
				return
			}
			w.(http.Flusher).Flush()

			if !healthStatus.IsRunning {
				if _, err := fmt.Fprintf(w, "healthCheck status: Process finished successfully\n"); err != nil {
					log.Printf("Error writing final message: %v", err)
				}
				w.(http.Flusher).Flush()
				cancel()
				return
			}
		case <-ctx.Done():
			if _, err := fmt.Fprintf(w, "healthCheck status: Monitoring cancelled\n"); err != nil {
				log.Printf("Error writing final message: %v", err)
			}
			w.(http.Flusher).Flush()
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
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var err error
	client, err = remote_process_client.New(reqBody.RemoteProcessServerAddress)
	if err != nil {
		http.Error(w, "Failed to create gRPC client", http.StatusInternalServerError)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	success, message, err := client.StopProcess(ctx, reqBody.ProcessID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to stop process: %v", err), http.StatusInternalServerError)
		return
	}

	response := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: success,
		Message: message,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
