package model

import (
	"dev.rubentxu.hodei-devops/protos/remote_worker"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
)

type ProcessOutput struct {
	ProcessID string
	Output    string
	IsError   bool
	Type      string
	Status    HealthStatus
}

type InspectResult struct {
	IsRunning       bool
	State           string
	AdditionalError error
}

type HealthStatus int32

const (
	UNKNOWN  HealthStatus = 0
	RUNNING  HealthStatus = 1
	HEALTHY  HealthStatus = 2
	ERROR    HealthStatus = 3
	STOPPED  HealthStatus = 4
	FINISHED HealthStatus = 5
	PENDING  HealthStatus = 6
	DONE     HealthStatus = 7
)

func (hs HealthStatus) String() string {

	switch hs {
	case PENDING:
		return "PENDING"
	case RUNNING:
		return "RUNNING"
	case HEALTHY:
		return "HEALTHY"
	case ERROR:
		return "ERROR"
	case STOPPED:
		return "STOPPED"
	case FINISHED:
		return "FINISHED"
	case DONE:
		return "DONE"
	default:
		return "UNKNOWN"
	}
}

func ConvertProtoProcessStatusToPorts(status remote_worker.ProcessStatus) HealthStatus {
	log.Printf("Convirtiendo status: %v", status)
	switch status {
	case remote_worker.ProcessStatus_UNKNOWN_PROCESS_STATUS:
		return UNKNOWN
	case remote_worker.ProcessStatus_RUNNING:
		return RUNNING
	case remote_worker.ProcessStatus_HEALTHY:
		return HEALTHY
	case remote_worker.ProcessStatus_ERROR:
		return ERROR
	case remote_worker.ProcessStatus_STOPPED:
		return STOPPED
	case remote_worker.ProcessStatus_FINISHED:
		return FINISHED
	default:
		log.Printf("Status desconocido: %v", status)
		return UNKNOWN
	}
}

// ProcessHealthStatus representa el estado de un proceso.
type ProcessHealthStatus struct {
	ProcessID string
	Status    HealthStatus
	Message   string
}

type ResourceConfig map[string]interface{}

// Métodos básicos de la interfaz
func (m ResourceConfig) GetType() string {
	return m.GetString("type", "")
}

func (m ResourceConfig) GetName() string {
	return m.GetString("name", "")
}

func (m ResourceConfig) GetDescription() string {
	return m.GetString("description", "")
}

// Métodos genéricos para obtener valores
func (m ResourceConfig) GetString(key string, defaultValue string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func (m ResourceConfig) GetInt(key string, defaultValue int) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			if num, err := strconv.Atoi(v); err == nil {
				return num
			}
		}
	}
	return defaultValue
}

func (m ResourceConfig) GetBool(key string, defaultValue bool) bool {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			if b, err := strconv.ParseBool(v); err == nil {
				return b
			}
		case int:
			return v != 0
		}
	}
	return defaultValue
}

func (m ResourceConfig) GetFloat(key string, defaultValue float64) float64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	}
	return defaultValue
}

// Métodos para establecer valores de manera segura
func (m ResourceConfig) Set(key string, value interface{}) {
	m[key] = value
}

// Métodos para manejar estructuras anidadas
func (m ResourceConfig) GetMap(key string) ResourceConfig {
	if val, ok := m[key]; ok {
		if mapVal, ok := val.(map[string]interface{}); ok {
			return ResourceConfig(mapVal)
		}
	}
	return make(ResourceConfig)
}

func (m ResourceConfig) GetStringSlice(key string) []string {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case []string:
			return v
		case []interface{}:
			result := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return []string{}
}

// Métodos de validación
func (m ResourceConfig) HasKey(key string) bool {
	_, exists := m[key]
	return exists
}

func (m ResourceConfig) IsEmpty() bool {
	return len(m) == 0
}

// Métodos de utilidad
func (m ResourceConfig) Clone() ResourceConfig {
	clone := make(ResourceConfig, len(m))
	for k, v := range m {
		clone[k] = v
	}
	return clone
}

func (m ResourceConfig) Merge(other ResourceConfig) {
	for k, v := range other {
		m[k] = v
	}
}

// Ejemplo de uso con validación
func (m ResourceConfig) Validate() error {
	required := []string{"type", "name"}
	var missingFields []string

	for _, field := range required {
		if !m.HasKey(field) || m.GetString(field, "") == "" {
			missingFields = append(missingFields, field)
		}
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("campos requeridos faltantes: %v", missingFields)
	}

	return nil
}

// Método para serializar a JSON
func (m ResourceConfig) ToJSON() (string, error) {
	bytes, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("error al serializar a JSON: %w", err)
	}
	return string(bytes), nil
}

// Método para crear desde JSON
func NewMapResourcePoolConfigFromJSON(jsonStr string) (ResourceConfig, error) {
	var m ResourceConfig
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return nil, fmt.Errorf("error al deserializar JSON: %w", err)
	}
	return m, nil
}
