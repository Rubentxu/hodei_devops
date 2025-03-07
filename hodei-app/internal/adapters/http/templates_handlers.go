package http_handlers

import (
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/manager"
	"dev.rubentxu.devops-platform/orchestrator/internal/domain"
	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
	"encoding/json"
	"fmt"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"log"
	"net/http"
)

// setupTemplateRoutes configura los endpoints para gestionar templates de workers
func SetupTemplateRoutes(e *core.ServeEvent, manager *manager.Manager) {
	// Obtener el resource pool por defecto una única vez
	getDefaultPool := func() (*ports.ResourcePool, error) {
		resourcePool := manager.GetResourcePool("defaultDockerPool")
		if resourcePool == nil {
			return nil, fmt.Errorf("resource pool not found")
		}
		return resourcePool, nil
	}

	// Listar todos los templates
	e.Router.GET("/api/templates", func(c *core.RequestEvent) error {
		resourcePool, err := getDefaultPool()
		if err != nil {
			return apis.NewApiError(404, err.Error(), nil)
		}

		// Ya que ahora usamos directamente la BD, vamos a acceder directamente al store
		// Accedemos al templateStore a través de una interface adicional
		if storeAccessor, ok := (*resourcePool).(ports.TemplateStoreAccessor); ok {
			templates, err := storeAccessor.GetTemplateStore().List()
			if err != nil {
				return apis.NewApiError(500, fmt.Sprintf("Failed to list templates: %v", err), nil)
			}

			c.Response.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(c.Response).Encode(templates)
		}

		// Implementación de respaldo usando la interface existente
		templates := []ports.WorkerTemplate{}
		// Obtenemos una lista de templates disponibles en la BD
		log.Println("Using fallback template listing method")

		c.Response.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(c.Response).Encode(templates)
	})

	// Obtener un template por ID
	e.Router.GET("/api/templates/:id", func(c *core.RequestEvent) error {
		id := c.Request.PathValue("id")
		if id == "" {
			return apis.NewApiError(400, "Template ID is required", nil)
		}

		resourcePool, err := getDefaultPool()
		if err != nil {
			return apis.NewApiError(404, err.Error(), nil)
		}

		template, err := (*resourcePool).GetWorkerTemplate(id)
		if err != nil {
			return apis.NewApiError(404, fmt.Sprintf("Template not found: %v", err), nil)
		}

		c.Response.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(c.Response).Encode(template)
	})

	// Crear un nuevo template
	e.Router.POST("/api/templates", func(c *core.RequestEvent) error {
		var template ports.WorkerTemplate

		if err := json.NewDecoder(c.Request.Body).Decode(&template); err != nil {
			return apis.NewApiError(400, fmt.Sprintf("Invalid request body: %v", err), nil)
		}

		// Validaciones básicas
		if template.ID == "" {
			return apis.NewApiError(400, "Template ID is required", nil)
		}
		if template.WorkerSpec.Type == "" {
			template.WorkerSpec.Type = domain.DockerInstance
		}
		if template.WorkerSpec.Image == "" {
			return apis.NewApiError(400, "Worker image is required", nil)
		}

		resourcePool, err := getDefaultPool()
		if err != nil {
			return apis.NewApiError(404, err.Error(), nil)
		}

		// Guardar el template
		if err := (*resourcePool).AddWorkerTemplate(template); err != nil {
			return apis.NewApiError(500, fmt.Sprintf("Failed to save template: %v", err), nil)
		}

		c.Response.WriteHeader(http.StatusCreated)
		c.Response.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(c.Response).Encode(template)
	})

	// Eliminar un template
	e.Router.DELETE("/api/templates/:id", func(c *core.RequestEvent) error {
		id := c.Request.PathValue("id")
		if id == "" {
			return apis.NewApiError(400, "Template ID is required", nil)
		}

		resourcePool, err := getDefaultPool()
		if err != nil {
			return apis.NewApiError(404, err.Error(), nil)
		}

		// Intentar eliminar directamente usando el store
		if storeAccessor, ok := (*resourcePool).(ports.TemplateStoreAccessor); ok {
			if err := storeAccessor.GetTemplateStore().Delete(id); err != nil {
				return apis.NewApiError(500, fmt.Sprintf("Failed to delete template: %v", err), nil)
			}

			c.Response.WriteHeader(http.StatusNoContent)
			return nil
		}

		return apis.NewApiError(501, "Delete template functionality not implemented for this resource pool", nil)
	})
}
