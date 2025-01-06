package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TestCase define la estructura de un caso de prueba
type TestCase struct {
	Name       string            `json:"name"`
	ProcessID  string            `json:"process_id"`
	Command    []string          `json:"command"`
	Env        map[string]string `json:"env,omitempty"`
	WorkingDir string            `json:"working_directory,omitempty"`
	Duration   time.Duration     // Duración para procesos largos
	Timeout    time.Duration     // Timeout para operaciones (stop, health check, etc.)
}

// RequestBody define la estructura del JSON para las peticiones
type RequestBody struct {
	RemoteProcessServerAddress string            `json:"remote_process_server_address"`
	Command                    []string          `json:"command"`
	ProcessID                  string            `json:"process_id"`
	CheckInterval              int64             `json:"check_interval"`
	Env                        map[string]string `json:"env,omitempty"`
	WorkingDirectory           string            `json:"working_directory,omitempty"`
	Timeout                    time.Duration     `json:"timeout,omitempty"` // Timeout en segundos
}

const serverAddr = "localhost:50051"
const apiBaseURL = "http://localhost:8080"

func main() {
	// Definir los casos de prueba
	testCases := []TestCase{
		{
			Name:      "Quick Process",
			ProcessID: "quick-process",
			Command:   []string{"echo", "Hello"},
			Timeout:   5 * time.Second,
		},
		{
			Name:      "Long Process",
			ProcessID: "long-process",
			Command:   []string{"bash", "-c", "sleep 30"},
			Duration:  35 * time.Second,
			Timeout:   45 * time.Second,
		},
		{
			Name:      "Resource Intensive",
			ProcessID: "resource-heavy",
			Command:   []string{"bash", "-c", "for i in {1..1000000}; do echo $i > /dev/null; done"},
			Duration:  20 * time.Second,
			Timeout:   60 * time.Second, // Proceso que puede necesitar más tiempo para detenerse
		},
		{
			Name:      "Network Process",
			ProcessID: "network-process",
			Command:   []string{"ping", "-c", "100", "google.com"},
			Duration:  15 * time.Second,
			Timeout:   10 * time.Second, // Proceso que debería responder rápido al SIGTERM
		},
	}

	// Ejecutar los tests en paralelo
	runTests(testCases)
}

func runTests(testCases []TestCase) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ejecutar cada caso de prueba en su propia goroutine
	for _, tc := range testCases {
		wg.Add(1)
		go func(tc TestCase) {
			defer wg.Done()
			runTest(ctx, tc)
		}(tc)
	}

	// Esperar a que todos los tests terminen
	wg.Wait()
}

func runTest(ctx context.Context, tc TestCase) {
	log.Printf("Starting test: %s (ProcessID: %s)", tc.Name, tc.ProcessID)

	// Crear un contexto cancelable para este test específico
	testCtx, cancelTest := context.WithCancel(ctx)
	defer cancelTest()

	// Crear los canales para la comunicación
	processDone := make(chan struct{})
	healthDone := make(chan struct{})

	// Usar timeout específico del test o valor por defecto
	timeout := tc.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Iniciar el proceso
	go runProcess(testCtx, tc, processDone)
	// Iniciar el health check
	go monitorHealth(testCtx, tc, healthDone)

	// Esperar según la duración especificada o usar un valor por defecto
	duration := tc.Duration
	if duration == 0 {
		duration = 10 * time.Second
	}

	select {
	case <-time.After(duration):
		// Detener el proceso y cancelar el contexto
		stopProcess(tc.ProcessID, timeout)
		cancelTest() // Cancelar el contexto para detener las goroutines
	case <-ctx.Done():
		log.Printf("Test cancelled: %s", tc.Name)
	}

	// Esperar a que terminen las goroutines
	<-processDone
	<-healthDone
	log.Printf("Test completed: %s", tc.Name)
}

func runProcess(ctx context.Context, tc TestCase, done chan<- struct{}) {
	defer close(done)

	url := fmt.Sprintf("%s/run", apiBaseURL)
	reqBody := RequestBody{
		RemoteProcessServerAddress: serverAddr,
		Command:                    tc.Command,
		ProcessID:                  tc.ProcessID,
		CheckInterval:              5,
		Env:                        tc.Env,
		WorkingDirectory:           tc.WorkingDir,
		Timeout:                    tc.Timeout,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[%s] Error marshaling request: %v", tc.ProcessID, err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[%s] Error creating request: %v", tc.ProcessID, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[%s] Error executing request: %v", tc.ProcessID, err)
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	// Aumentar el tamaño del buffer para manejar líneas largas
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] Process monitoring cancelled", tc.ProcessID)
			return
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					log.Printf("[%s] Error reading output: %v", tc.ProcessID, err)
				}
				return
			}
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				log.Printf("[%s] Process output: %s", tc.ProcessID, data)
			}
		}
	}
}

func monitorHealth(ctx context.Context, tc TestCase, done chan<- struct{}) {
	defer close(done)

	url := fmt.Sprintf("%s/health", apiBaseURL)
	reqBody := RequestBody{
		RemoteProcessServerAddress: serverAddr,
		ProcessID:                  tc.ProcessID,
		CheckInterval:              5,
		Timeout:                    tc.Timeout,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[%s] Error marshaling health request: %v", tc.ProcessID, err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[%s] Error creating health request: %v", tc.ProcessID, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[%s] Error executing health request: %v", tc.ProcessID, err)
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	// Aumentar el tamaño del buffer para manejar líneas largas
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] Health monitoring cancelled", tc.ProcessID)
			return
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					log.Printf("[%s] Error reading health status: %v", tc.ProcessID, err)
				}
				return
			}
			line := scanner.Text()
			if strings.HasPrefix(line, "healthCheck") {
				log.Printf("[%s] Health status: %s", tc.ProcessID, line)
			}
		}
	}
}

func stopProcess(processID string, timeout time.Duration) {
	if timeout == 0 {
		timeout = 30 * time.Second // valor por defecto
	}

	reqBody := RequestBody{
		RemoteProcessServerAddress: serverAddr,
		ProcessID:                  processID,
		Timeout:                    timeout,
	}

	// Crear un cliente HTTP con timeout configurable
	client := &http.Client{
		Timeout: timeout + 5*time.Second, // añadir un margen al timeout
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[%s] Error marshaling stop request: %v", processID, err)
		return
	}

	resp, err := client.Post(apiBaseURL+"/stop", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[%s] Error stopping process: %v", processID, err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[%s] Error reading response body: %v", processID, err)
		return
	}

	// Intentar decodificar como JSON
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		// Si falla, tratar el cuerpo como mensaje de error
		log.Printf("[%s] Error stopping process: %s", processID, string(body))
		return
	}

	if !response.Success {
		log.Printf("[%s] Error stopping process: %s", processID, response.Message)
		return
	}

	// Verificar que el proceso realmente se detuvo con más paciencia
	maxRetries := 15 // Aumentar el número de reintentos
	for i := 0; i < maxRetries; i++ {
		if isProcessStopped(processID, timeout) {
			log.Printf("[%s] Process stopped successfully", processID)
			return
		}
		time.Sleep(2 * time.Second) // Esperar más entre intentos
	}
	log.Printf("[%s] Warning: Process may not have stopped completely", processID)
}

func isProcessStopped(processID string, timeout time.Duration) bool {
	if timeout == 0 {
		timeout = 5 * time.Second // valor por defecto
	}

	reqBody := RequestBody{
		RemoteProcessServerAddress: serverAddr,
		ProcessID:                  processID,
		CheckInterval:              1,
		Timeout:                    timeout,
	}

	client := &http.Client{
		Timeout: timeout,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[%s] Error marshaling health check request: %v", processID, err)
		return false
	}

	resp, err := client.Post(apiBaseURL+"/health", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[%s] Error executing health check request: %v", processID, err)
		return false
	}
	defer resp.Body.Close()

	var healthResponse struct {
		ProcessID string `json:"process_id"`
		IsRunning bool   `json:"is_running"`
		Status    string `json:"status"`
		IsHealthy bool   `json:"is_healthy"`
		Timestamp string `json:"timestamp"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&healthResponse); err != nil {
		// Solo loguear el error si no es un error de EOF
		if err != io.EOF {
			log.Printf("[%s] Error decoding health check response: %v", processID, err)
		}
		return false
	}

	// Log para debug
	log.Printf("[%s] Health check response: %+v", processID, healthResponse)

	return !healthResponse.IsRunning
}

func init() {
	// Configurar el formato del log para incluir timestamp
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
}
