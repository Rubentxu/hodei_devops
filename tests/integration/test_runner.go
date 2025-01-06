package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"
)

type TestRunner struct {
	BaseURL string
}

func NewTestRunner(baseURL string) *TestRunner {
	return &TestRunner{BaseURL: baseURL}
}

func (tr *TestRunner) startProcess(url string, tc TestCase) (*http.Response, error) {
	reqBody := ProcessRequest{
		RemoteProcessServerAddress: "server:50051",
		Command:                    tc.Command,
		ProcessID:                  tc.ProcessID,
		CheckInterval:              5,
		Env:                        tc.Env,
		WorkingDirectory:           tc.WorkingDir,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request: %v", err)
	}

	return resp, nil
}

func (tr *TestRunner) monitorOutput(t *testing.T, reader io.Reader, tc TestCase) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			log.Printf("[%s] Output: %s", tc.ProcessID, data)
		}
	}
}

func (tr *TestRunner) monitorHealth(t *testing.T, tc TestCase) {
	url := fmt.Sprintf("%s/health", tr.BaseURL)
	reqBody := ProcessRequest{
		RemoteProcessServerAddress: "server:50051",
		ProcessID:                  tc.ProcessID,
		CheckInterval:              5,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		t.Errorf("Error marshaling health request: %v", err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		t.Errorf("Error creating health request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("Error executing health request: %v", err)
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "healthCheck") {
			log.Printf("[%s] Health: %s", tc.ProcessID, line)
		}
	}
}

func (tr *TestRunner) stopProcess(processID string) error {
	url := fmt.Sprintf("%s/stop", tr.BaseURL)
	reqBody := ProcessRequest{
		RemoteProcessServerAddress: "server:50051",
		ProcessID:                  processID,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("error marshaling stop request: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error stopping process: %v", err)
	}
	defer resp.Body.Close()

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("error decoding response: %v", err)
	}

	if !response.Success {
		return fmt.Errorf("failed to stop process: %s", response.Message)
	}

	return nil
}

func (tr *TestRunner) RunTests(t *testing.T, testCases []TestCase) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Iniciar el proceso
			processURL := fmt.Sprintf("%s/run", tr.BaseURL)
			resp, err := tr.startProcess(processURL, tc)
			if err != nil {
				t.Fatalf("Failed to start process: %v", err)
			}
			defer resp.Body.Close()

			// Monitorear la salida y el estado de salud
			go tr.monitorOutput(t, resp.Body, tc)
			go tr.monitorHealth(t, tc)

			// Esperar el tiempo especificado
			time.Sleep(tc.Duration)

			// Detener el proceso
			err = tr.stopProcess(tc.ProcessID)
			if err != nil {
				t.Errorf("Failed to stop process: %v", err)
			}
		})
	}
}
