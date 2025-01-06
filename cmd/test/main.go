package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
}

// RequestBody define la estructura del JSON para las peticiones
type RequestBody struct {
	RemoteProcessServerAddress string            `json:"remote_process_server_address"`
	Command                    []string          `json:"command"`
	ProcessID                  string            `json:"process_id"`
	CheckInterval              int64             `json:"check_interval"`
	Env                        map[string]string `json:"env,omitempty"`
	WorkingDirectory           string            `json:"working_directory,omitempty"`
}

const serverAddr = "localhost:50051"
const apiBaseURL = "http://localhost:8080"

func main() {
	// Definir los casos de prueba
	testCases := []TestCase{
		{
			Name:      "Ping Google",
			ProcessID: "ping-google",
			Command:   []string{"ping", "-c", "5", "google.com"},
		},
		{
			Name:       "List Directory",
			ProcessID:  "list-dir",
			Command:    []string{"ls", "-la"},
			WorkingDir: "/home",
		},
		{
			Name:      "Loop Process",
			ProcessID: "loop-10",
			Command:   []string{"bash", "-c", "for i in {1..10}; do echo Looping... iteration $i; sleep 1; done"},
		},
		{
			Name:      "Environment Variables",
			ProcessID: "env",
			Command:   []string{"printenv"},
			Env: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
		},
		{
			Name:      "Long Running Process",
			ProcessID: "long-process-1",
			Command:   []string{"bash", "-c", "i=1; while true; do echo \"Long process 1... iteration $i\"; i=$((i+1)); sleep 5; done"},
			Duration:  30 * time.Second,
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
		stopProcess(tc.ProcessID)
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

	reqBody := RequestBody{
		RemoteProcessServerAddress: serverAddr,
		Command:                    tc.Command,
		ProcessID:                  tc.ProcessID,
		CheckInterval:              5,
		Env:                        tc.Env,
		WorkingDirectory:           tc.WorkingDir,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Error marshaling request: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiBaseURL+"/run", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error executing request: %v", err)
		return
	}
	defer resp.Body.Close()

	// Leer el stream de eventos
	reader := bufio.NewReader(resp.Body)
	for {
		// Crear un canal para el timeout de lectura
		readChan := make(chan string, 1)
		errChan := make(chan error, 1)

		// Leer en una goroutine separada
		go func() {
			line, err := reader.ReadString('\n')
			if err != nil {
				errChan <- err
				return
			}
			readChan <- line
		}()

		// Esperar la lectura o timeout
		select {
		case <-ctx.Done():
			log.Printf("[%s] Process stream cancelled", tc.ProcessID)
			return
		case line := <-readChan:
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				data = strings.TrimSpace(data)
				log.Printf("[%s] Process output: %s", tc.ProcessID, data)
			}
		case err := <-errChan:
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				log.Printf("[%s] Error reading stream: %v", tc.ProcessID, err)
			}
			return
		case <-time.After(time.Second):
			continue // Timeout, intentar leer de nuevo
		}
	}
}

func monitorHealth(ctx context.Context, tc TestCase, done chan<- struct{}) {
	defer close(done)

	reqBody := RequestBody{
		RemoteProcessServerAddress: serverAddr,
		ProcessID:                  tc.ProcessID,
		CheckInterval:              5,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Error marshaling health request: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiBaseURL+"/health", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating health request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error executing health request: %v", err)
		return
	}
	defer resp.Body.Close()

	// Leer el stream de eventos de salud
	reader := bufio.NewReader(resp.Body)
	for {
		// Crear un canal para el timeout de lectura
		readChan := make(chan string, 1)
		errChan := make(chan error, 1)

		// Leer en una goroutine separada
		go func() {
			line, err := reader.ReadString('\n')
			if err != nil {
				errChan <- err
				return
			}
			readChan <- line
		}()

		// Esperar la lectura o timeout
		select {
		case <-ctx.Done():
			log.Printf("[%s] Health stream cancelled", tc.ProcessID)
			return
		case line := <-readChan:
			if strings.HasPrefix(line, "healthCheck") {
				line = strings.TrimSpace(line)
				log.Printf("[%s] Health status: %s", tc.ProcessID, line)
			}
		case err := <-errChan:
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				log.Printf("[%s] Error reading health stream: %v", tc.ProcessID, err)
			}
			return
		case <-time.After(time.Second):
			continue // Timeout, intentar leer de nuevo
		}
	}
}

func stopProcess(processID string) {
	reqBody := RequestBody{
		RemoteProcessServerAddress: serverAddr,
		ProcessID:                  processID,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Error marshaling stop request: %v", err)
		return
	}

	resp, err := http.Post(apiBaseURL+"/stop", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error stopping process: %v", err)
		return
	}
	defer resp.Body.Close()

	// Leer y mostrar la respuesta
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("Error decoding stop response: %v", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] Error stopping process: status code %d, message: %s",
			processID, resp.StatusCode, response.Message)
	} else {
		log.Printf("[%s] Process stopped successfully: %s", processID, response.Message)
	}
}

func init() {
	// Configurar el formato del log para incluir timestamp
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
}
