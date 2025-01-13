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
	"sort"
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

// Agregar esta nueva estructura para el monitoreo global
type ProcessStatus struct {
	ProcessID string
	Status    string
	Output    string
	Timestamp time.Time
}

// Agregar esta estructura para eventos de salud
type HealthEvent struct {
	ProcessID string
	Status    string
	Timestamp time.Time
	Resources map[string]interface{}
}

const serverAddr = "localhost:50051"
const apiBaseURL = "http://localhost:8080"

func main() {
	testCases := []TestCase{
		// Tests básicos de conectividad y comunicación
		{
			Name:      "Basic Echo",
			ProcessID: "echo-basic",
			Command:   []string{"echo", "Hello"},
			Duration:  5 * time.Second,
		},

		// Tests de latencia y timeouts
		{
			Name:      "Slow Process with Network Delay",
			ProcessID: "slow-process",
			Command:   []string{"bash", "-c", "sleep 1 && echo 'Step 1' && sleep 2 && echo 'Step 2'"},
			Duration:  10 * time.Second,
		},

		// Tests de carga y recursos
		{
			Name:      "CPU Intensive",
			ProcessID: "cpu-heavy",
			Command: []string{"bash", "-c", `
				for i in {1..1000000}; do 
					if [ $((i % 100000)) -eq 0 ]; then
						echo "Processed $i iterations"
					fi
					echo $i > /dev/null
				done
			`},
			Duration: 20 * time.Second,
		},
		{
			Name:      "CPU Load Monitor",
			ProcessID: "cpu-monitor",
			Command: []string{"bash", "-c", `
				for i in {1..20}; do 
					echo "CPU Load Check $i:"
					top -b -n1 | head -n 3
					sleep 1
				done
			`},
			Duration: 25 * time.Second,
		},
		{
			Name:      "System Resources",
			ProcessID: "sys-resources",
			Command: []string{"bash", "-c", `
				echo "=== Memory Info ==="
				free -h
				echo "=== CPU Info ==="
				lscpu | grep "CPU(s):"
				echo "=== Process Count ==="
				ps aux | wc -l
				echo "=== System Load ==="
				uptime
			`},
			Duration: 10 * time.Second,
		},
		{
			Name:      "Memory Intensive",
			ProcessID: "memory-heavy",
			Command:   []string{"bash", "-c", "dd if=/dev/zero of=/tmp/test bs=1M count=1024 && rm /tmp/test"},
			Duration:  30 * time.Second,
		},

		// Tests de E/S y bloqueo
		{
			Name:      "IO Blocking",
			ProcessID: "io-block",
			Command:   []string{"bash", "-c", "dd if=/dev/urandom of=/dev/null bs=1M count=1000"},
			Duration:  15 * time.Second,
		},

		// Tests de manejo de errores y casos límite
		{
			Name:      "Invalid Command",
			ProcessID: "invalid-cmd",
			Command:   []string{"nonexistentcommand"},
			Duration:  5 * time.Second,
		},
		{
			Name:      "Permission Denied",
			ProcessID: "perm-denied",
			Command:   []string{"cat", "/root/secret"},
			Duration:  5 * time.Second,
		},

		// Tests de variables de entorno y contexto
		{
			Name:      "Environment Variables",
			ProcessID: "env-vars",
			Command:   []string{"bash", "-c", "echo $TEST_VAR1-$TEST_VAR2"},
			Env: map[string]string{
				"TEST_VAR1": "value1",
				"TEST_VAR2": "value2",
			},
			Duration: 5 * time.Second,
		},

		// Tests de red y conectividad
		{
			Name:      "Network Connectivity",
			ProcessID: "network-test",
			Command:   []string{"curl", "-s", "http://example.com"},
			Duration:  10 * time.Second,
		},
		{
			Name:      "DNS Resolution",
			ProcessID: "dns-test",
			Command:   []string{"dig", "+short", "google.com"},
			Duration:  10 * time.Second,
		},

		// Tests de procesos anidados y grupos
		{
			Name:      "Process Tree",
			ProcessID: "process-tree",
			Command: []string{"bash", "-c",
				`child_proc() { sleep 2; }; 
				 child_proc & 
				 child_proc & 
				 wait`},
			Duration: 15 * time.Second,
		},

		// Tests de límites y recursos
		{
			Name:      "File Descriptors",
			ProcessID: "fd-test",
			Command: []string{"bash", "-c",
				`for i in $(seq 1 1000); do 
					exec {fd}>/dev/null; 
					eval "exec $fd>&-"; 
				done`},
			Duration: 10 * time.Second,
		},

		// Tests de recuperación y resiliencia
		{
			Name:      "Process Recovery",
			ProcessID: "recovery-test",
			Command: []string{"bash", "-c",
				`trap 'echo "Recovering..."; sleep 1' ERR;
				 false;
				 echo "Recovered"`},
			Duration: 10 * time.Second,
		},

		// Tests de concurrencia
		{
			Name:      "Concurrent Operations",
			ProcessID: "concurrent-ops",
			Command: []string{"bash", "-c",
				`for i in {1..5}; do 
					(echo "Thread $i"; sleep 1) & 
				done; 
				wait`},
			Duration: 15 * time.Second,
		},

		// Los últimos dos tests son para parada de procesos en paralelo
		{
			Name:      "Long Running Process 1",
			ProcessID: "long-process-1",
			Command: []string{"bash", "-c",
				`trap 'echo "Graceful shutdown 1"' TERM;
				 i=1; 
				 while true; do 
					echo "Process 1 - iteration $i"; 
					i=$((i+1)); 
					sleep 5; 
				 done`},
			Duration: 30 * time.Second,
			Timeout:  45 * time.Second,
		},
		{
			Name:      "Long Running Process 2",
			ProcessID: "long-process-2",
			Command: []string{"bash", "-c",
				`trap 'echo "Graceful shutdown 2"' TERM;
				 i=1; 
				 while true; do 
					echo "Process 2 - iteration $i"; 
					i=$((i+1)); 
					sleep 5; 
				 done`},
			Duration: 30 * time.Second,
			Timeout:  45 * time.Second,
		},

		// Tests específicos de MonitorHealth
		{
			Name:      "Health Check Basic",
			ProcessID: "health-basic",
			Command:   []string{"sleep", "10"},
			Duration:  12 * time.Second,
		},
		{
			Name:      "Health Check State Changes",
			ProcessID: "health-states",
			Command: []string{"bash", "-c", `
				echo "Starting..."
				sleep 2
				echo "Running..."
				sleep 2
				echo "Finishing..."
				exit 0
			`},
			Duration: 10 * time.Second,
		},
		{
			Name:      "Health Check Error State",
			ProcessID: "health-error",
			Command: []string{"bash", "-c", `
				echo "Starting..."
				sleep 2
				echo "About to fail..."
				exit 1
			`},
			Duration: 8 * time.Second,
		},
		{
			Name:      "Health Check Resource Usage",
			ProcessID: "health-resources",
			Command: []string{"bash", "-c", `
				echo "Starting CPU work..."
				for i in {1..3}; do
					dd if=/dev/zero of=/dev/null bs=1M count=1024
					echo "CPU iteration $i complete"
					sleep 1
				done
			`},
			Duration: 15 * time.Second,
		},
		{
			Name:      "Health Check Signal Handling",
			ProcessID: "health-signals",
			Command: []string{"bash", "-c", `
				trap 'echo "Received SIGTERM"; exit 0' TERM
				echo "Starting..."
				while true; do
					echo "Still running..."
					sleep 1
				done
			`},
			Duration: 10 * time.Second,
		},
		{
			Name:      "Health Check Rapid Status Changes",
			ProcessID: "health-rapid",
			Command: []string{"bash", "-c", `
				for i in {1..5}; do
					echo "State $i"
					sleep 0.5
					if [ $i -eq 3 ]; then
						(dd if=/dev/zero of=/dev/null bs=1M count=512 &)
					fi
				done
			`},
			Duration: 8 * time.Second,
		},
		{
			Name:      "Health Check with Subprocess",
			ProcessID: "health-subprocess",
			Command: []string{"bash", "-c", `
				echo "Parent starting..."
				(sleep 2; echo "Child 1 running..."; sleep 2) &
				(sleep 3; echo "Child 2 running..."; sleep 2) &
				wait
				echo "All processes complete"
			`},
			Duration: 12 * time.Second,
		},
		{
			Name:      "Health Check Network Status",
			ProcessID: "health-network",
			Command: []string{"bash", "-c", `
				echo "Checking network..."
				ping -c 3 8.8.8.8
				curl -s https://api.github.com/zen
				echo "Network checks complete"
			`},
			Duration: 15 * time.Second,
		},
		{
			Name:      "Health Check Memory Pressure",
			ProcessID: "health-memory",
			Command: []string{"bash", "-c", `
				echo "Allocating memory..."
				dd if=/dev/zero of=/tmp/test bs=1M count=512
				echo "Memory allocated"
				sleep 2
				rm /tmp/test
				echo "Memory freed"
			`},
			Duration: 10 * time.Second,
		},
		{
			Name:      "Health Check Concurrent Operations",
			ProcessID: "health-concurrent",
			Command: []string{"bash", "-c", `
				echo "Starting concurrent operations..."
				(dd if=/dev/zero of=/dev/null bs=1M count=256) &
				(ping -c 5 8.8.8.8) &
				(sleep 3; echo "Operation 3") &
				wait
				echo "All operations complete"
			`},
			Duration: 15 * time.Second,
		},
	}

	runTests(testCases)
}

// Modificar la función runTests para usar un único monitor de salud
func runTests(testCases []TestCase) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Iniciar el monitor de salud global que durará toda la ejecución
	healthCtx, healthCancel := context.WithCancel(ctx)
	defer healthCancel()

	healthDone := make(chan struct{})
	go func() {
		defer close(healthDone)
		monitorAllProcessesHealth(healthCtx)
	}()

	// Ejecutar tests normales en serie
	normalTests := testCases[:len(testCases)-2]
	for _, tc := range normalTests {
		testCtx, cancel := context.WithTimeout(ctx, tc.Duration)
		runTestUntilCompletion(testCtx, tc)
		cancel()
		time.Sleep(2 * time.Second)
	}

	// Ejecutar los tests de parada en paralelo
	var wg sync.WaitGroup
	stopTests := testCases[len(testCases)-2:]
	for _, tc := range stopTests {
		wg.Add(1)
		go func(tc TestCase) {
			defer wg.Done()
			runTestWithStop(ctx, tc)
		}(tc)
	}

	wg.Wait()
	time.Sleep(2 * time.Second)

	// Cancelar el monitor de salud y esperar a que termine
	healthCancel()
	<-healthDone
}

// Nueva función para monitorear la salud de todos los procesos
func monitorAllProcessesHealth(ctx context.Context) {
	log.Printf("🏥 Starting global health monitoring")

	url := fmt.Sprintf("%s/health", apiBaseURL)
	reqBody := RequestBody{
		RemoteProcessServerAddress: serverAddr,
		ProcessID:                  "global-monitor",
		CheckInterval:              1,
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, createJSONBody(reqBody))
	if err != nil {
		log.Printf("Error creating health monitor request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error starting health monitor: %v", err)
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// Mantener estado de todos los procesos
	processStates := make(map[string]*HealthEvent)
	var stateMutex sync.RWMutex

	// Ticker para mostrar resumen periódico
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Health monitoring stopped")
			return
		case <-ticker.C:
			stateMutex.RLock()
			if len(processStates) > 0 {
				showHealthSummary(processStates, &stateMutex)
			}
			stateMutex.RUnlock()
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					log.Printf("Error reading health status: %v", err)
				}
				return
			}

			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				// Mostrar datos de salud
				data := strings.TrimPrefix(line, "data: ")
				var healthData map[string]interface{}
				if err := json.Unmarshal([]byte(data), &healthData); err == nil {
					if pid, ok := healthData["process_id"].(string); ok {
						log.Printf("💓 [%s] Health Data: %s", pid, data)

						event := HealthEvent{
							ProcessID: pid,
							Timestamp: time.Now(),
							Resources: healthData,
							Status:    fmt.Sprintf("%v", healthData["status"]),
						}

						stateMutex.Lock()
						processStates[pid] = &event
						stateMutex.Unlock()
					}
				}
			} else if strings.HasPrefix(line, "healthCheck") {
				// Mostrar eventos de healthcheck
				status := strings.TrimPrefix(line, "healthCheck ")
				parts := strings.SplitN(status, ":", 2)
				if len(parts) > 1 {
					pid := strings.TrimSpace(parts[0])
					state := strings.TrimSpace(parts[1])
					log.Printf("🔍 [%s] Health Status: %s", pid, state)

					event := HealthEvent{
						ProcessID: pid,
						Timestamp: time.Now(),
						Status:    state,
						Resources: map[string]interface{}{
							"state": state,
						},
					}

					stateMutex.Lock()
					processStates[pid] = &event
					stateMutex.Unlock()
				}
			}
		}
	}
}

// Modificar la función runTestUntilCompletion para usar el monitor centralizado
func runTestUntilCompletion(ctx context.Context, tc TestCase) {
	log.Printf("Starting test: %s (ProcessID: %s)", tc.Name, tc.ProcessID)

	processDone := make(chan struct{})
	go runProcess(ctx, tc, processDone)

	select {
	case <-processDone:
		log.Printf("Test completed naturally: %s", tc.Name)
	case <-ctx.Done():
		log.Printf("[%s] Test timed out", tc.ProcessID)
	}
}

// Modificar runTestWithStop para usar el monitor centralizado
func runTestWithStop(ctx context.Context, tc TestCase) {
	log.Printf("Starting test with stop: %s (ProcessID: %s)", tc.Name, tc.ProcessID)

	testCtx, cancelTest := context.WithCancel(ctx)
	defer cancelTest()

	processDone := make(chan struct{})
	go runProcess(testCtx, tc, processDone)

	select {
	case <-time.After(tc.Duration):
		stopProcess(tc.ProcessID, tc.Timeout)
		cancelTest()
	case <-ctx.Done():
		log.Printf("Test cancelled: %s", tc.Name)
		stopProcess(tc.ProcessID, tc.Timeout)
	}

	<-processDone
	log.Printf("Test completed with stop: %s", tc.Name)
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

	// Crear un cliente HTTP con timeout
	client := &http.Client{
		Timeout: tc.Duration + 5*time.Second,
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

func monitorHealth(ctx context.Context, tc TestCase, done chan<- struct{}) {
	defer close(done)

	url := fmt.Sprintf("%s/health", apiBaseURL)
	reqBody := RequestBody{
		RemoteProcessServerAddress: serverAddr,
		ProcessID:                  tc.ProcessID,
		CheckInterval:              1,
		Timeout:                    tc.Timeout,
	}

	// Crear un cliente HTTP con timeout
	client := &http.Client{
		Timeout: tc.Duration + 5*time.Second,
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
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[%s] Error executing health request: %v", tc.ProcessID, err)
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			line := scanner.Text()
			if strings.HasPrefix(line, "healthCheck") {
				log.Printf("[%s] Health status: %s", tc.ProcessID, line)
				if strings.Contains(line, "Process finished") ||
					strings.Contains(line, "Process stopped") {
					return
				}
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

// Agregar estas funciones helper
func createJSONBody(reqBody RequestBody) *bytes.Buffer {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Error marshaling request body: %v", err)
		return bytes.NewBuffer(nil)
	}
	return bytes.NewBuffer(jsonData)
}

func parseHealthEvent(processID string, line string) HealthEvent {
	event := HealthEvent{
		ProcessID: processID,
		Timestamp: time.Now(),
		Resources: make(map[string]interface{}),
	}

	if strings.HasPrefix(line, "data: ") {
		data := strings.TrimPrefix(line, "data: ")
		// Intentar parsear el JSON de los recursos
		var resources map[string]interface{}
		if err := json.Unmarshal([]byte(data), &resources); err == nil {
			event.Resources = resources
			event.Status = "Data received"
		} else {
			event.Status = data
		}
	} else if strings.HasPrefix(line, "healthCheck") {
		event.Status = strings.TrimPrefix(line, "healthCheck ")
		// Extraer información adicional si está disponible
		if strings.Contains(event.Status, "running") {
			event.Resources["state"] = "running"
		} else if strings.Contains(event.Status, "stopped") {
			event.Resources["state"] = "stopped"
		} else if strings.Contains(event.Status, "error") {
			event.Resources["state"] = "error"
			// Extraer mensaje de error si existe
			if parts := strings.Split(event.Status, ":"); len(parts) > 1 {
				event.Resources["error"] = strings.TrimSpace(parts[1])
			}
		}
	}

	return event
}

func showHealthSummary(states map[string]*HealthEvent, mutex *sync.RWMutex) {
	mutex.RLock()
	defer mutex.RUnlock()

	if len(states) == 0 {
		return
	}

	// Limpiar la pantalla para mejor visualización
	fmt.Print("\033[H\033[2J")

	var running, stopped, failed int
	var sb strings.Builder

	sb.WriteString("\n🏥 Health Status Summary:\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Ordenar los procesos por ID para una visualización consistente
	var pids []string
	for pid := range states {
		pids = append(pids, pid)
	}
	sort.Strings(pids)

	for _, pid := range pids {
		state := states[pid]
		statusEmoji := "⏳" // default
		switch {
		case strings.Contains(state.Status, "running"):
			statusEmoji = "✅"
			running++
		case strings.Contains(state.Status, "stopped"):
			statusEmoji = "⏹️"
			stopped++
		case strings.Contains(state.Status, "error"):
			statusEmoji = "❌"
			failed++
		}

		sb.WriteString(fmt.Sprintf("%s Process: %s\n", statusEmoji, pid))
		sb.WriteString(fmt.Sprintf("   ├── Status: %s\n", state.Status))
		sb.WriteString(fmt.Sprintf("   ├── Last Update: %s\n", time.Since(state.Timestamp).Round(time.Second)))

		// Mostrar recursos si existen
		if len(state.Resources) > 0 {
			sb.WriteString("   └── Resources:\n")
			for k, v := range state.Resources {
				sb.WriteString(fmt.Sprintf("      ├── %s: %v\n", k, v))
			}
		}
		sb.WriteString("\n")
	}

	// Mostrar barra de progreso
	total := running + stopped + failed
	sb.WriteString(fmt.Sprintf("\nProgress: [%d/%d]\n", stopped+failed, total))
	sb.WriteString(fmt.Sprintf("├── Running: %d 🟢\n", running))
	sb.WriteString(fmt.Sprintf("├── Stopped: %d 🔵\n", stopped))
	sb.WriteString(fmt.Sprintf("└── Failed: %d 🔴\n", failed))

	log.Print(sb.String())
}
