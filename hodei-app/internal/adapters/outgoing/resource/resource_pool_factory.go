package resource

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"fmt"
	"k8s.io/apimachinery/pkg/util/rand"
	"log"
	"time"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
)

// DefaultResourcePoolFactory implementa la interfaz ResourcePoolFactory
// y es capaz de crear diferentes tipos de ResourcePools según la configuración
type DefaultResourcePoolFactory struct{}

// NewDefaultResourcePoolFactory crea una nueva instancia de la fábrica de ResourcePools
func NewDefaultResourcePoolFactory() ports.ResourcePoolFactory {
	return &DefaultResourcePoolFactory{}
}

// CreateResourcePool crea una instancia de ResourcePool a partir de una configuración
func (f *DefaultResourcePoolFactory) CreateResourcePool(definition *model.ResourcePoolDef) (ports.ResourcePool, error) {
	// Verificar que existe un tipo en la configuración
	poolType := definition.Spec.Type

	// Verificar que existe un nombre en la configuración
	name := definition.Metadata.Name
	if name == "" {
		return nil, fmt.Errorf("falta o es inválido el campo 'name' en la definición del resource pool")
	}

	// Crear el ResourcePool según el tipo
	switch poolType {
	case "docker":
		return f.createDockerResourcePool(name, definition)
	case "kubernetes":
		return f.createKubernetesResourcePool(name, definition)
	default:
		return nil, fmt.Errorf("unsupported resource pool type: %s", poolType)
	}
}

// createDockerResourcePool crea un DockerResourcePool a partir de la configuración
func (f *DefaultResourcePoolFactory) createDockerResourcePool(id string, poolDef *model.ResourcePoolDef) (ports.ResourcePool, error) {

	dockerConfig := DockerResourcesPoolConfig{
		Type:        "docker",
		Name:        id,
		Description: poolDef.Metadata.Description,
		Host:        poolDef.Spec.PoolConfig.GetString("host", "unix:///var/run/docker.sock"),
	}

	log.Printf("Creating Docker resource pool with poolDef: %+v", dockerConfig)
	return NewDockerResourcePool(id, dockerConfig)
}

func (f *DefaultResourcePoolFactory) CreateDefaultResourcePool() (ports.ResourcePool, error) {

	id := "defaultDockerPool"
	defaultConfig := DockerResourcesPoolConfig{
		Type:        "docker",
		Name:        "defaultDockerPool",
		Description: "Pool de recursos Docker por defecto",
		Host:        "unix:///var/run/docker.sock",
		NetworkName: "default-orchestrator-network",
		StopDelay:   5 * time.Second,
		TLSVerify:   false,
	}

	return NewDockerResourcePool(id, defaultConfig)
}

// createKubernetesResourcePool crea un KubernetesResourcePool a partir de la configuración
func (f *DefaultResourcePoolFactory) createKubernetesResourcePool(id string, poolDef *model.ResourcePoolDef) (ports.ResourcePool, error) {

	k8sConfig := KubernetesResoucesPoolConfig{
		Type:        "kubernetes",
		Name:        id,
		Description: poolDef.Metadata.Description,
		Namespace:   poolDef.Spec.PoolConfig.GetString("namespace", "default"),
		KubeConfig:  poolDef.Spec.PoolConfig.GetString("kubeConfig", ""),
		InCluster:   poolDef.Spec.PoolConfig.GetBool("inCluster", true),
		Labels:      poolDef.Metadata.Labels,
		Annotations: poolDef.Metadata.Annotations,
	}

	log.Printf("Creating Kubernetes resource pool with config: %+v", k8sConfig)
	return NewKubernetesResourcePool(id, k8sConfig)
}

// getStringOrDefault obtiene un valor string del mapa o devuelve un valor por defecto
func getStringOrDefault(config map[string]interface{}, key, defaultValue string) string {
	if value, ok := config[key].(string); ok {
		return value
	}
	return defaultValue
}

// getBoolOrDefault obtiene un valor booleano del mapa o devuelve un valor por defecto
func getBoolOrDefault(config map[string]interface{}, key string, defaultValue bool) bool {
	if value, ok := config[key].(bool); ok {
		return value
	}
	return defaultValue
}

// getMapOrDefault obtiene un mapa string->string del mapa o devuelve un mapa vacío
func getMapOrDefault(config map[string]interface{}, key string) map[string]string {
	if value, ok := config[key].(map[string]interface{}); ok {
		result := make(map[string]string)
		for k, v := range value {
			if strValue, ok := v.(string); ok {
				result[k] = strValue
			}
		}
		return result
	}
	return make(map[string]string)
}

func GetStringOrDefault(config map[string]interface{}, key, defaultValue string) string {
	if value, ok := config[key].(string); ok {
		return value
	}
	return defaultValue
}

func GetBoolOrDefault(config map[string]interface{}, key string, defaultValue bool) bool {
	if value, ok := config[key].(bool); ok {
		return value
	}
	return defaultValue
}

func GetMapOrDefault(config map[string]interface{}, key string) map[string]interface{} {
	if value, ok := config[key].(map[string]interface{}); ok {
		return value
	}
	return make(map[string]interface{})
}

func GetStringMapOrDefault(config map[string]interface{}, key string) map[string]string {
	result := make(map[string]string)
	if value, ok := config[key].(map[string]interface{}); ok {
		for k, v := range value {
			if str, ok := v.(string); ok {
				result[k] = str
			}
		}
	}
	return result
}

func GetDurationOrDefault(config map[string]interface{}, key string, defaultValue time.Duration) time.Duration {
	if value, ok := config[key].(float64); ok {
		return time.Duration(value) * time.Second
	}
	return defaultValue
}

func generateURLSafeHash(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
