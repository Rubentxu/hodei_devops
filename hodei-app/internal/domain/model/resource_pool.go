package model

import (
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator"
	"strconv"
)

var _ AggregateRoot = (*ResourcePoolDef)(nil)

type ResourcePoolDef struct {
	ID       AggregateID        `json:"id" validate:"required"`
	Metadata Metadata           `json:"metadata" validate:"required"`
	Spec     ResourcePoolSpec   `json:"spec" validate:"required"`
	Status   ResourcePoolStatus `json:"status" validate:"required"`
}

func (r *ResourcePoolDef) GetID() AggregateID {
	return r.ID
}

type ResourcePoolSpec struct {
	PoolID     string         `json:"poolID" validate:"required"`
	Type       string         `json:"type" validate:"required,oneof=Kubernetes Docker VM"`
	PoolConfig ResourceConfig `json:"config,omitempty" validate:"required"`
}

type ResourcePoolStatus struct {
	State string `json:"state" validate:"required,oneof=PENDING ACTIVE INACTIVE ERROR DELETED"`
}

func (r *ResourcePoolDef) Validate() error {
	validate := validator.New()

	// Registrar validación personalizada si fuera necesaria
	if err := validate.RegisterValidation("pooltype", ValidatePoolType); err != nil {
		return err
	}

	return validate.Struct(r)
}

// Función auxiliar de validación si necesitas lógica personalizada
func ValidatePoolType(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	validTypes := map[string]bool{
		"Kubernetes": true,
		"Docker":     true,
		"VM":         true,
	}
	return validTypes[value]
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
