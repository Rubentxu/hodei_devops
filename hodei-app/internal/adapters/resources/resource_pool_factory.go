package resources

import (
	"fmt"
	"log"
	"time"

	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
)

// DefaultResourcePoolFactory implementa la interfaz ResourcePoolFactory
// y es capaz de crear diferentes tipos de ResourcePools según la configuración
type DefaultResourcePoolFactory struct{}

// NewDefaultResourcePoolFactory crea una nueva instancia de la fábrica de ResourcePools
func NewDefaultResourcePoolFactory() ports.ResourcePoolFactory {
	return &DefaultResourcePoolFactory{}
}

// CreateResourcePool crea una instancia de ResourcePool a partir de una configuración
func (f *DefaultResourcePoolFactory) CreateResourcePool(config map[string]interface{}, templateStore ports.Store[ports.WorkerTemplate]) (ports.ResourcePool, error) {
	// Verificar que existe un tipo en la configuración
	poolType, ok := config["type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'type' in resource pool config")
	}

	// Verificar que existe un nombre en la configuración
	name, ok := config["name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'name' in resource pool config")
	}

	// Crear el ResourcePool según el tipo
	switch poolType {
	case "docker":
		return f.createDockerResourcePool(name, config, templateStore)
	case "kubernetes":
		return f.createKubernetesResourcePool(name, config, templateStore)
	default:
		return nil, fmt.Errorf("unsupported resource pool type: %s", poolType)
	}
}

// createDockerResourcePool crea un DockerResourcePool a partir de la configuración
func (f *DefaultResourcePoolFactory) createDockerResourcePool(id string, config map[string]interface{}, templateStore ports.Store[ports.WorkerTemplate]) (ports.ResourcePool, error) {
	// Convertir el mapa genérico a la configuración específica
	dockerConfig := DockerResourcesPoolConfig{
		Type:        "docker",
		Name:        id,
		Description: getStringOrDefault(config, "description", ""),
		Host:        getStringOrDefault(config, "host", "unix:///var/run/docker.sock"),
	}

	log.Printf("Creating Docker resource pool with config: %+v", dockerConfig)

	// Crear el DockerResourcePool con el cliente y la configuración
	return NewDockerResourcePool(id, templateStore, dockerConfig)
}

func (f *DefaultResourcePoolFactory) CreateDefaultResourcePool() (ports.ResourcePoolConfig, error) {
	// Convertir el mapa genérico a la configuración específica
	defaultConfig := &DockerResourcesPoolConfig{
		Type:        "docker",
		Name:        "defaultDockerPool",
		Description: "Pool de recursos Docker por defecto",
		Host:        "unix:///var/run/docker.sock",
		NetworkName: "default-orchestrator-network",
		StopDelay:   5 * time.Second,
		TLSVerify:   false,
	}

	return defaultConfig, nil
}

// createKubernetesResourcePool crea un KubernetesResourcePool a partir de la configuración
func (f *DefaultResourcePoolFactory) createKubernetesResourcePool(id string, config map[string]interface{}, templateStore ports.Store[ports.WorkerTemplate]) (ports.ResourcePool, error) {
	// Convertir el mapa genérico a la configuración específica
	k8sConfig := KubernetesResoucesPoolConfig{
		Type:        "kubernetes",
		Name:        id,
		Description: getStringOrDefault(config, "description", ""),
		Namespace:   getStringOrDefault(config, "namespace", "default"),
		KubeConfig:  getStringOrDefault(config, "kubeConfig", ""),
		InCluster:   getBoolOrDefault(config, "inCluster", false),
		Labels:      getMapOrDefault(config, "labels"),
		Annotations: getMapOrDefault(config, "annotations"),
	}

	log.Printf("Creating Kubernetes resource pool with config: %+v", k8sConfig)

	// Crear el KubernetesResourcePool con la configuración
	return NewKubernetesResourcePool(id, k8sConfig, templateStore)
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
