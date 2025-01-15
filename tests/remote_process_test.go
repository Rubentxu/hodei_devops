package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// ProcessTestCase define la estructura de un caso de prueba
type ProcessTestCase struct {
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

var (
	apiBaseURL = "http://localhost:8080"
	serverAddr = "localhost:50051"
)

func TestProcessExecution(t *testing.T) {
	for _, tc := range ProcessTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tc.Duration)
			defer cancel()

			processDone := make(chan struct{})
			go runProcess(ctx, tc, processDone)

			select {
			case <-processDone:
				t.Logf("Test completed: %s", tc.Name)
			case <-ctx.Done():
				t.Errorf("Test timed out: %s", tc.Name)
			}
		})
	}
}

func runProcess(ctx context.Context, tc ProcessTestCase, done chan<- struct{}) {
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

	req, err := http.NewRequestWithContext(ctx, "POST", url, createJSONBody(reqBody))
	if err != nil {
		log.Printf("[%s] Error creating request: %v", tc.ProcessID, err)
		return
	}

	// Agregar el token JWT al header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("JWT_TOKEN")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	// Crear un cliente HTTP con timeout
	client := &http.Client{
		Timeout: tc.Duration + 5*time.Second,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[%s] Error marshaling request: %v", tc.ProcessID, err)
		return
	}

	req, err = http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[%s] Error creating request: %v", tc.ProcessID, err)
		return
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[%s] Error executing request: %v", tc.ProcessID, err)
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	// Canal para manejar el timeout de lectura
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				log.Printf("[%s] Process output: %s", tc.ProcessID, data)
			}
		}
	}()

	select {
	case <-ctx.Done():
		log.Printf("[%s] Process monitoring cancelled", tc.ProcessID)
		return
	case <-readDone:
		if err := scanner.Err(); err != nil {
			log.Printf("[%s] Error reading output: %v", tc.ProcessID, err)
		}
		return
	}
}

// Agregar estas funciones helper
func createJSONBody(reqBody RequestBody) *bytes.Buffer {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Error marshaling request body: %v", err)
		return bytes.NewBuffer(nil)
	}
	return bytes.NewBuffer(jsonData)
}
