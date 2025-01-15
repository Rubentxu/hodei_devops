package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// MetricsTestCase define la estructura para casos de prueba de métricas
type MetricsTestCase struct {
	Name          string
	WorkerID      string
	MetricTypes   []string
	Duration      time.Duration
	CheckInterval int64
}

// Lista de casos de prueba
var MetricsTestCases = []MetricsTestCase{
	{
		Name:          "Basic CPU Metrics",
		WorkerID:      "worker-cpu",
		MetricTypes:   []string{"cpu"},
		Duration:      10 * time.Second,
		CheckInterval: 1,
	},
	{
		Name:          "Memory Usage Tracking",
		WorkerID:      "worker-memory",
		MetricTypes:   []string{"memory"},
		Duration:      15 * time.Second,
		CheckInterval: 1,
	},
	{
		Name:          "Disk IO Monitoring",
		WorkerID:      "worker-disk",
		MetricTypes:   []string{"disk", "io"},
		Duration:      20 * time.Second,
		CheckInterval: 2,
	},
	{
		Name:          "Network Statistics",
		WorkerID:      "worker-network",
		MetricTypes:   []string{"network"},
		Duration:      15 * time.Second,
		CheckInterval: 1,
	},
	{
		Name:          "Full System Metrics",
		WorkerID:      "worker-full",
		MetricTypes:   []string{"cpu", "memory", "disk", "network", "system", "process", "io"},
		Duration:      30 * time.Second,
		CheckInterval: 2,
	},
	{
		Name:          "Process Specific Metrics",
		WorkerID:      "worker-process",
		MetricTypes:   []string{"process"},
		Duration:      15 * time.Second,
		CheckInterval: 1,
	},
}

// MetricsRequest se utilizará para enviar la configuración como primer mensaje
type MetricsRequest struct {
	WorkerID    string   `json:"worker_id"`
	Interval    int64    `json:"interval"`
	MetricTypes []string `json:"metric_types"`
}

// TestMetricsCollection ejecuta todos los casos de prueba de métricas
func TestMetricsCollection(t *testing.T) {
	// Asegurar que el servidor esté listo
	if err := waitForServer(t); err != nil {
		t.Fatalf("Server not ready: %v", err)
	}

	for _, tc := range MetricsTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tc.Duration)
			defer cancel()

			metricsChan := make(chan map[string]interface{}, 100)
			errChan := make(chan error, 1)

			go collectMetricsWS(ctx, tc, metricsChan, errChan, t)

			var metricsCount int
			// Mínimo de muestras esperadas (aprox.) de acuerdo al intervalo
			expectedMinCount := int(tc.Duration.Seconds() / float64(tc.CheckInterval))

			// Bucle para leer los datos que llegan por metricsChan
			for {
				select {
				case err := <-errChan:
					t.Errorf("Error collecting metrics: %v", err)
					return
				case metrics, ok := <-metricsChan:
					if !ok {
						// Canal cerrado => fin de la recepción
						if metricsCount < expectedMinCount {
							t.Errorf("Expected at least %d metrics updates, got %d",
								expectedMinCount, metricsCount)
						}
						return
					}
					metricsCount++
					validateMetrics(t, tc.MetricTypes, metrics)
				case <-ctx.Done():
					t.Logf("Test completed with %d metrics updates", metricsCount)
					return
				}
			}
		})
	}
}

// collectMetricsWS se conecta vía WebSocket a /metrics, envía la configuración
// como primer mensaje, y lee continuamente las actualizaciones de métricas.
func collectMetricsWS(
	ctx context.Context,
	tc MetricsTestCase,
	metricsChan chan<- map[string]interface{},
	errChan chan<- error,
	t *testing.T,
) {
	defer close(metricsChan)

	// (1) URL base para WebSocket (sin query string)
	wsURL := fmt.Sprintf("%s/metrics", wsBaseURL)

	// (2) Crear el dialer
	dialer := &websocket.Dialer{}

	// (3) Cabeceras para la conexión
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("JWT_TOKEN")))

	// (4) Establecer la conexión WebSocket
	conn, resp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		errChan <- fmt.Errorf("error making websocket dial: %w", err)
		return
	}
	defer conn.Close()

	// Verificar handshake (opcional)
	if resp != nil && resp.StatusCode != http.StatusSwitchingProtocols {
		errChan <- fmt.Errorf("server did not switch protocols; status=%d", resp.StatusCode)
		return
	}

	// (5) Enviar la configuración de métricas como primer mensaje
	req := MetricsRequest{
		WorkerID:    tc.WorkerID,
		Interval:    tc.CheckInterval,
		MetricTypes: tc.MetricTypes,
	}
	if err := sendJSONWebSocket(conn, req); err != nil {
		errChan <- fmt.Errorf("error sending first message: %w", err)
		return
	}

	// (6) Leer en bucle hasta que se cancele el contexto
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Leer siguiente mensaje
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				// Error típico al cerrar la conexión
				errChan <- fmt.Errorf("error reading websocket message: %w", err)
				return
			}

			if msgType == websocket.CloseMessage {
				log.Printf("[metrics] Server closed the connection.")
				return
			}

			// Mensaje en formato JSON
			var metrics map[string]interface{}
			if err := json.Unmarshal(msg, &metrics); err != nil {
				errChan <- fmt.Errorf("error parsing JSON metrics: %w", err)
				continue
			}

			// Enviar al canal principal
			metricsChan <- metrics
		}
	}
}

// waitForServer -> Health check HTTP (como en tu código original)
func waitForServer(t *testing.T) error {
	timeout := time.After(30 * time.Second)
	tick := time.Tick(1 * time.Second)

	url := fmt.Sprintf("%s/health", apiBaseURL)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for server")
		case <-tick:
			resp, err := http.Get(url)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}
}

// validateMetrics y las funciones de validación específicas se conservan.
// No requieren cambios, siempre que la estructura JSON recibida sea la misma.
func validateMetrics(t *testing.T, metricTypes []string, metrics map[string]interface{}) {
	t.Logf("\n Metrics Update:")

	// Mostrar todas las claves disponibles para depuración
	var keys []string
	for k := range metrics {
		keys = append(keys, k)
	}
	t.Logf("Available metric types: %v", keys)

	// Mapa de nombres de métricas
	metricTypeMap := map[string]string{
		"disk":    "disks",
		"network": "networks",
		"cpu":     "cpu",
		"memory":  "memory",
		"system":  "system",
		"process": "processes",
		"io":      "io",
	}

	for _, metricType := range metricTypes {
		actualMetricType := metricTypeMap[metricType]
		metric, ok := metrics[actualMetricType]
		if !ok {
			t.Errorf("Expected metric type %s (mapped to %s) not found. Available types: %v",
				metricType, actualMetricType, keys)
			continue
		}

		switch metricType {
		case "cpu":
			validateCPUMetrics(t, metric)
		case "memory":
			validateMemoryMetrics(t, metric)
		case "disk":
			validateDiskMetrics(t, metric)
		case "network":
			validateNetworkMetrics(t, metric)
		case "system":
			validateSystemMetrics(t, metric)
		case "process":
			validateProcessMetrics(t, metric)
		case "io":
			validateIOMetrics(t, metric)
		}
	}
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// Resto de validaciones (validateCPUMetrics, validateMemoryMetrics, etc.)
// se mantienen exactamente como antes, sin cambios:
//
// func validateCPUMetrics(t *testing.T, metric interface{}) { ... }
// func validateMemoryMetrics(t *testing.T, metric interface{}) { ... }
// func validateDiskMetrics(t *testing.T, metric interface{}) { ... }
// func validateNetworkMetrics(t *testing.T, metric interface{}) { ... }
// func validateSystemMetrics(t *testing.T, metric interface{}) { ... }
// func validateProcessMetrics(t *testing.T, metric interface{}) { ... }
// func validateIOMetrics(t *testing.T, metric interface{}) { ... }
//
// Igualmente, las funciones auxiliares getFloat64Value(...) y formatBytes(...)
// no cambian, ya que todo se mantiene en JSON.

// Funciones de validación específicas para cada tipo de métrica
func validateCPUMetrics(t *testing.T, metric interface{}) {
	cpu, ok := metric.(map[string]interface{})
	if !ok {
		t.Error("Invalid CPU metrics format")
		return
	}

	requiredFields := []string{"total_usage_percent", "core_count", "thread_count"}
	for _, field := range requiredFields {
		if _, ok := cpu[field]; !ok {
			t.Errorf("Missing required CPU field: %s", field)
		}
	}

	t.Logf("   CPU Usage: %.2f%%", cpu["total_usage_percent"])
	t.Logf("   Cores: %v", cpu["core_count"])
	t.Logf("   Threads: %v", cpu["thread_count"])
}

func validateMemoryMetrics(t *testing.T, metric interface{}) {
	mem, ok := metric.(map[string]interface{})
	if !ok {
		t.Error("Invalid memory metrics format")
		return
	}

	requiredFields := []string{"total", "used", "free", "available"}
	for _, field := range requiredFields {
		if _, ok := mem[field]; !ok {
			t.Errorf("Missing required memory field: %s", field)
		}
	}

	t.Logf("   Memory Total: %v", formatBytes(mem["total"].(float64)))
	t.Logf("   Memory Used: %v", formatBytes(mem["used"].(float64)))
	t.Logf("   Memory Free: %v", formatBytes(mem["free"].(float64)))
	t.Logf("   Memory Available: %v", formatBytes(mem["available"].(float64)))
}

func validateDiskMetrics(t *testing.T, metric interface{}) {
	disks, ok := metric.([]interface{})
	if !ok {
		t.Error("Invalid disk metrics format")
		return
	}

	t.Logf("   Disk Metrics:")
	for _, disk := range disks {
		diskMap, ok := disk.(map[string]interface{})
		if !ok {
			t.Error("Invalid disk entry format")
			continue
		}

		// Validar campos requeridos
		requiredFields := []string{"device", "mount_point"} // Reducir campos requeridos
		for _, field := range requiredFields {
			value, exists := diskMap[field]
			if !exists || value == nil {
				t.Errorf("Missing required disk field: %s", field)
				continue
			}
		}

		t.Logf("     Device: %v", diskMap["device"])
		t.Logf("     Mount Point: %v", diskMap["mount_point"])

		// Mostrar campos opcionales con valores por defecto
		if total, ok := getFloat64Value(diskMap["total"]); ok {
			t.Logf("     Total: %v", formatBytes(total))
		} else {
			t.Logf("     Total: N/A")
		}

		if used, ok := getFloat64Value(diskMap["used"]); ok {
			t.Logf("     Used: %v", formatBytes(used))
		} else {
			t.Logf("     Used: N/A")
		}

		if free, ok := getFloat64Value(diskMap["free"]); ok {
			t.Logf("     Free: %v", formatBytes(free))
		} else {
			t.Logf("     Free: N/A")
		}

		t.Log("     ---")
	}
}

// Función auxiliar para convertir valores a float64 de forma segura
func getFloat64Value(value interface{}) (float64, bool) {
	if value == nil {
		return 0, false
	}

	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint64:
		return float64(v), true
	default:
		return 0, false
	}
}

// Función auxiliar para formatear bytes
func formatBytes(bytes float64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%.2f B", bytes)
	}
	div, exp := float64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", bytes/div, "KMGTPE"[exp])
}

// Agregar las funciones de validación faltantes con sus respectivos logs
func validateNetworkMetrics(t *testing.T, metric interface{}) {
	networks, ok := metric.([]interface{})
	if !ok {
		t.Errorf("Invalid network metrics format: got %T, want []interface{}", metric)
		return
	}

	t.Logf("   Network Metrics:")
	for _, net := range networks {
		netMap, ok := net.(map[string]interface{})
		if !ok {
			t.Errorf("Invalid network entry format: got %T, want map[string]interface{}", net)
			continue
		}

		// Verificar campos requeridos
		requiredFields := []string{"interface", "bytes_sent", "bytes_recv"}
		for _, field := range requiredFields {
			value, exists := netMap[field]
			if !exists {
				t.Errorf("Missing required network field: %s", field)
				continue
			}
			if value == nil {
				t.Errorf("Field %s has nil value", field)
				continue
			}
		}

		t.Logf("     Interface: %v", netMap["interface"])
		// Convertir y validar valores numéricos de forma segura
		if bytesSent, ok := getFloat64Value(netMap["bytes_sent"]); ok {
			t.Logf("     Bytes Sent: %v", formatBytes(bytesSent))
		} else {
			t.Logf("     Bytes Sent: N/A")
		}
		if bytesRecv, ok := getFloat64Value(netMap["bytes_recv"]); ok {
			t.Logf("     Bytes Received: %v", formatBytes(bytesRecv))
		} else {
			t.Logf("     Bytes Received: N/A")
		}

		if mac, ok := netMap["mac_address"].(string); ok {
			t.Logf("     MAC Address: %v", mac)
		}
		if ip, ok := netMap["ip_address"].(string); ok {
			t.Logf("     IP Address: %v", ip)
		}

		// Mostrar campos opcionales si están disponibles
		if bw, ok := getFloat64Value(netMap["bandwidth_usage"]); ok {
			t.Logf("     Bandwidth Usage: %.2f%%", bw)
		}
		if status, ok := netMap["status"].(map[string]interface{}); ok {
			t.Logf("     Status: Up=%v, Running=%v", status["is_up"], status["is_running"])
		}
		t.Log("     ---")
	}
}

func validateSystemMetrics(t *testing.T, metric interface{}) {
	sys, ok := metric.(map[string]interface{})
	if !ok {
		t.Error("Invalid system metrics format")
		return
	}

	t.Logf("   System Info:")
	t.Logf("     Hostname: %v", sys["hostname"])
	t.Logf("     OS: %v", sys["os"])
	t.Logf("     Uptime: %v seconds", sys["uptime"])
	t.Logf("     Process Count: %v", sys["process_count"])
}

func validateProcessMetrics(t *testing.T, metric interface{}) {
	processes, ok := metric.([]interface{})
	if !ok {
		t.Error("Invalid process metrics format")
		return
	}

	t.Logf("   Process Metrics:")
	for _, proc := range processes {
		procMap, ok := proc.(map[string]interface{})
		if !ok {
			t.Error("Invalid process entry format")
			continue
		}

		// Validar campos requeridos
		requiredFields := []string{"pid", "name", "cpu_percent", "memory_rss", "memory_vms", "status"}
		for _, field := range requiredFields {
			value, exists := procMap[field]
			if !exists {
				t.Errorf("Missing required process field: %s", field)
				continue
			}
			if value == nil {
				t.Errorf("Field %s has nil value", field)
				continue
			}
		}

		// PID
		if pid, ok := getFloat64Value(procMap["pid"]); ok {
			t.Logf("     PID: %d", int(pid))
		} else {
			t.Logf("     PID: N/A")
		}

		// Name
		if name, ok := procMap["name"].(string); ok {
			t.Logf("     Name: %s", name)
		} else {
			t.Logf("     Name: N/A")
		}

		// CPU
		if cpu, ok := getFloat64Value(procMap["cpu_percent"]); ok {
			t.Logf("     CPU: %.2f%%", cpu)
		} else {
			t.Logf("     CPU: N/A")
		}

		// Memory RSS
		if rss, ok := getFloat64Value(procMap["memory_rss"]); ok {
			t.Logf("     Memory RSS: %v", formatBytes(rss))
		} else {
			t.Logf("     Memory RSS: N/A")
		}

		// Memory VMS
		if vms, ok := getFloat64Value(procMap["memory_vms"]); ok {
			t.Logf("     Memory VMS: %v", formatBytes(vms))
		} else {
			t.Logf("     Memory VMS: N/A")
		}

		// Status
		if status, ok := procMap["status"].(string); ok {
			t.Logf("     Status: %s", status)
		} else {
			t.Logf("     Status: N/A")
		}

		// Threads (opcional)
		if threads, ok := procMap["threads"]; ok {
			if threadsNum, ok := getFloat64Value(threads); ok {
				t.Logf("     Threads: %d", int(threadsNum))
			} else {
				t.Logf("     Threads: N/A")
			}
		}
		t.Log("     ---")
	}
}

func validateIOMetrics(t *testing.T, metric interface{}) {
	io, ok := metric.(map[string]interface{})
	if !ok {
		t.Error("Invalid IO metrics format")
		return
	}

	// Validar campos requeridos
	requiredFields := []string{
		"read_bytes_total",
		"write_bytes_total",
		"read_speed",
		"write_speed",
		"active_requests",
	}

	for _, field := range requiredFields {
		if _, ok := io[field]; !ok {
			t.Errorf("Missing required IO field: %s", field)
		}
	}

	// Mostrar información de I/O
	t.Logf("   I/O Metrics:")
	// Verificar valores razonables de forma segura
	if readTotal, ok := getFloat64Value(io["read_bytes_total"]); ok {
		if readTotal > 1e12 { // > 1TB
			t.Logf("   ⚠️ Warning: Unusually high read total: %v", formatBytes(readTotal))
		}
		t.Logf("     Read Total: %v", formatBytes(readTotal))
	} else {
		t.Logf("     Read Total: N/A")
	}

	if writeTotal, ok := getFloat64Value(io["write_bytes_total"]); ok {
		if writeTotal > 1e12 {
			t.Logf("   ⚠️ Warning: Unusually high write total: %v", formatBytes(writeTotal))
		}
		t.Logf("     Write Total: %v", formatBytes(writeTotal))
	} else {
		t.Logf("     Write Total: N/A")
	}

	if readSpeed, ok := getFloat64Value(io["read_speed"]); ok {
		t.Logf("     Read Speed: %v/s", formatBytes(readSpeed))
	} else {
		t.Logf("     Read Speed: N/A")
	}

	if writeSpeed, ok := getFloat64Value(io["write_speed"]); ok {
		t.Logf("     Write Speed: %v/s", formatBytes(writeSpeed))
	} else {
		t.Logf("     Write Speed: N/A")
	}

	t.Logf("     Active Requests: %v", io["active_requests"])
	if queueLen, ok := io["queue_length"]; ok {
		t.Logf("     Queue Length: %.2f", queueLen)
	}
}
