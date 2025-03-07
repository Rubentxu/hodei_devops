package http_handlers

import (
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/manager"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/resources"
	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
	"encoding/json"
	"fmt"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"net/http"
	"time"
)

// setupResourcePoolRoutes configura los endpoints para gestionar ResourcePools
func SetupResourcePoolRoutes(
	e *core.ServeEvent,
	resourcePoolManager *resources.ResourcePoolManager,
	manager *manager.Manager,
) {
	// Listar todas las configuraciones de ResourcePool
	e.Router.GET("/api/resource-pools", func(c *core.RequestEvent) error {
		configs, err := resourcePoolManager.ListConfigs()
		if err != nil {
			return apis.NewApiError(500, fmt.Sprintf("Error listing resource pool configs: %v", err), nil)
		}

		c.Response.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(c.Response).Encode(configs)
	})

	// Obtener una configuración específica
	e.Router.GET("/api/resource-pools/:name", func(c *core.RequestEvent) error {
		name := c.Request.PathValue("name")
		if name == "" {
			return apis.NewApiError(400, "Resource pool name is required", nil)
		}

		config, err := resourcePoolManager.GetConfig(name)
		if err != nil {
			return apis.NewApiError(404, fmt.Sprintf("Resource pool config not found: %v", err), nil)
		}

		c.Response.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(c.Response).Encode(config)
	})

	// Crear/actualizar una configuración
	e.Router.POST("/api/resource-pools", func(c *core.RequestEvent) error {
		var config map[string]interface{}

		if err := json.NewDecoder(c.Request.Body).Decode(&config); err != nil {
			return apis.NewApiError(400, fmt.Sprintf("Invalid request body: %v", err), nil)
		}

		// Validar campos requeridos
		poolType, ok := config["type"].(string)
		if !ok || poolType == "" {
			return apis.NewApiError(400, "Resource pool type is required", nil)
		}

		name, ok := config["name"].(string)
		if !ok || name == "" {
			return apis.NewApiError(400, "Resource pool name is required", nil)
		}

		// Adaptar la configuración según el tipo
		var poolConfig ports.ResourcePoolConfig

		switch poolType {
		case "docker":
			dockerConfig := &resources.DockerResourcesPoolConfig{
				Type:           poolType,
				Name:           name,
				Description:    resources.GetStringOrDefault(config, "description", ""),
				Host:           resources.GetStringOrDefault(config, "host", "unix:///var/run/docker.sock"),
				NetworkName:    resources.GetStringOrDefault(config, "networkName", ""),
				StopDelay:      resources.GetDurationOrDefault(config, "stopDelay", 10*time.Second),
				TLSVerify:      resources.GetBoolOrDefault(config, "tlsVerify", false),
				DockerCertPath: resources.GetStringOrDefault(config, "dockerCertPath", ""),
			}
			poolConfig = dockerConfig
		case "kubernetes":
			k8sConfig := &resources.KubernetesResoucesPoolConfig{
				Type:        poolType,
				Name:        name,
				Description: resources.GetStringOrDefault(config, "description", ""),
				Namespace:   resources.GetStringOrDefault(config, "namespace", "default"),
				KubeConfig:  resources.GetStringOrDefault(config, "kubeConfig", ""),
				InCluster:   resources.GetBoolOrDefault(config, "inCluster", false),
				Labels:      resources.GetStringMapOrDefault(config, "labels"),
				Annotations: resources.GetStringMapOrDefault(config, "annotations"),
			}
			poolConfig = k8sConfig
		default:
			return apis.NewApiError(400, fmt.Sprintf("Unsupported resource pool type: %s", poolType), nil)
		}

		// Guardar la configuración
		if err := resourcePoolManager.SaveConfig(poolConfig); err != nil {
			return apis.NewApiError(500, fmt.Sprintf("Error saving resource pool config: %v", err), nil)
		}

		c.Response.WriteHeader(http.StatusCreated)
		c.Response.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(c.Response).Encode(config)
	})

	// Eliminar una configuración
	e.Router.DELETE("/api/resource-pools/:name", func(c *core.RequestEvent) error {
		name := c.Request.PathValue("name")
		if name == "" {
			return apis.NewApiError(400, "Resource pool name is required", nil)
		}

		// Verificar si el ResourcePool está activo
		if pool := manager.GetResourcePool(name); pool != nil {
			return apis.NewApiError(400, "Cannot delete an active resource pool, deactivate it first", nil)
		}

		// Eliminar la configuración
		if err := resourcePoolManager.DeleteConfig(name); err != nil {
			return apis.NewApiError(500, fmt.Sprintf("Error deleting resource pool config: %v", err), nil)
		}

		c.Response.WriteHeader(http.StatusNoContent)
		return nil
	})

	// Activar un ResourcePool
	e.Router.POST("/api/resource-pools/:name/activate", func(c *core.RequestEvent) error {
		name := c.Request.PathValue("name")
		if name == "" {
			return apis.NewApiError(400, "Resource pool name is required", nil)
		}

		// Verificar si el ResourcePool ya está activo
		if pool := manager.GetResourcePool(name); pool != nil {
			return apis.NewApiError(400, "Resource pool is already active", nil)
		}

		// Crear el ResourcePool
		err := resourcePoolManager.CreateResourcePool(name)
		if err != nil {
			return apis.NewApiError(500, fmt.Sprintf("Error creating resource pool: %v", err), nil)
		}

		c.Response.WriteHeader(http.StatusOK)
		c.Response.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(c.Response).Encode(map[string]string{
			"message": fmt.Sprintf("Resource pool %s activated successfully", name),
		})
	})

	// Desactivar un ResourcePool
	e.Router.POST("/api/resource-pools/:name/deactivate", func(c *core.RequestEvent) error {
		name := c.Request.PathValue("name")
		if name == "" {
			return apis.NewApiError(400, "Resource pool name is required", nil)
		}

		// Verificar si el ResourcePool está activo
		if pool := manager.GetResourcePool(name); pool == nil {
			return apis.NewApiError(400, "Resource pool is not active", nil)
		}

		// Eliminar el ResourcePool del manager
		if err := resourcePoolManager.UnregisterActivePool(name); err != nil {
			return apis.NewApiError(500, fmt.Sprintf("Error removing resource pool: %v", err), nil)
		}

		c.Response.WriteHeader(http.StatusOK)
		c.Response.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(c.Response).Encode(map[string]string{
			"message": fmt.Sprintf("Resource pool %s deactivated successfully", name),
		})
	})
}
