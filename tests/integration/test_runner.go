package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-time.After(5 * time.Second):
			t.Logf("[%s] Timeout reading output", tc.ProcessID)
			return
		default:
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				t.Logf("[%s] Output: %s", tc.ProcessID, data)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		t.Logf("[%s] Error reading output: %v", tc.ProcessID, err)
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
		t.Errorf("[%s] Error marshaling health request: %v", tc.ProcessID, err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		t.Errorf("[%s] Error creating health request: %v", tc.ProcessID, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("[%s] Error executing health request: %v", tc.ProcessID, err)
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-time.After(5 * time.Second):
			t.Logf("[%s] Timeout reading health status", tc.ProcessID)
			return
		default:
			line := scanner.Text()
			if strings.HasPrefix(line, "healthCheck") {
				t.Logf("[%s] Health: %s", tc.ProcessID, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		t.Logf("[%s] Error reading health status: %v", tc.ProcessID, err)
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
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tc.Duration+10*time.Second)
			defer cancel()

			processURL := fmt.Sprintf("%s/run", tr.BaseURL)
			resp, err := tr.startProcess(processURL, tc)
			if err != nil {
				t.Fatalf("Failed to start process: %v", err)
			}
			defer resp.Body.Close()

			outputDone := make(chan struct{})
			healthDone := make(chan struct{})

			go func() {
				defer close(outputDone)
				tr.monitorOutput(t, resp.Body, tc)
			}()

			go func() {
				defer close(healthDone)
				tr.monitorHealth(t, tc)
			}()

			select {
			case <-ctx.Done():
				t.Logf("[%s] Context cancelled or timeout reached", tc.ProcessID)
			case <-outputDone:
				t.Logf("[%s] Output monitoring finished", tc.ProcessID)
			case <-healthDone:
				t.Logf("[%s] Health monitoring finished", tc.ProcessID)
			}

			err = tr.stopProcess(tc.ProcessID)
			if err != nil {
				t.Errorf("Failed to stop process: %v", err)
			}

			select {
			case <-outputDone:
			case <-time.After(5 * time.Second):
				t.Logf("[%s] Timeout waiting for output monitoring to finish", tc.ProcessID)
			}

			select {
			case <-healthDone:
			case <-time.After(5 * time.Second):
				t.Logf("[%s] Timeout waiting for health monitoring to finish", tc.ProcessID)
			}
		})
	}
}
