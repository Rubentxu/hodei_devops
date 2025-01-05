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
	RemoteProcessServerAddress string   `json:"remote_process_server_address"`
	Command                    []string `json:"command"`
	ProcessID                  string   `json:"process_id"`
	CheckInterval              int64    `json:"check_interval"`
}

var client *remote_process_client.Client

func main() {
	http.HandleFunc("/run", runHandler)
	http.HandleFunc("/health", healthHandler)

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
	go runProcess(ctx, client, config, w, &wg)

	wg.Wait()
}

func runProcess(ctx context.Context, client *remote_process_client.Client, config Config, w http.ResponseWriter, wg *sync.WaitGroup) {
	defer wg.Done()

	// Create a channel to capture the output
	outputChan := make(chan *pb.ProcessOutput)

	// Start the process and redirect its output to the channel
	go func() {
		if err := client.StartProcess(ctx, config.ProcessID, config.Command, nil, outputChan); err != nil {
			log.Printf("Error in StartProcess: %v", err)
		}
	}()

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Stream the output to the HTTP response
	for output := range outputChan {
		if _, err := fmt.Fprintf(w, "data: %s\n\n", output.Output); err != nil {
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

	var wg sync.WaitGroup

	wg.Add(1)
	go monitorProcess(ctx, client, config, &wg)

	wg.Wait()
}

func monitorProcess(ctx context.Context, client *remote_process_client.Client, config Config, wg *sync.WaitGroup) {
	defer wg.Done()
	healthChan := make(chan *pb.HealthStatus)
	defer close(healthChan)

	go func() {
		if err := client.MonitorHealth(ctx, config.ProcessID, config.CheckInterval, healthChan); err != nil {
			log.Printf("Error in MonitorHealth: %v", err)
		}
	}()

	for healthStatus := range healthChan {
		log.Printf("Health status: %v", healthStatus)
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
