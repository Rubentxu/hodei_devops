package utils

import (
	"net/http"
	"os"
	"path/filepath"

	http_handlers "dev.rubentxu.devops-platform/orchestrator/internal/adapters/http"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/manager"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/resources"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/websockets"

	"github.com/pocketbase/pocketbase/core"
	httpSwagger "github.com/swaggo/http-swagger"

	_ "dev.rubentxu.devops-platform/orchestrator/docs"
)

// SetupRoutes configura todas las rutas del servidor
func SetupRoutes(
	e *core.ServeEvent,
	wsHandler *websockets.WSHandler,
	manager *manager.Manager,
	resourcePoolManager *resources.ResourcePoolManager,
) {
	// Obtener directorio base
	baseDir, _ := filepath.Abs(".")

	// WebSocket endpoint
	e.Router.GET("/ws", func(c *core.RequestEvent) error {
		wsHandler.HandleConnection(c.Response, c.Request)
		return nil
	})

	// Health endpoint
	e.Router.GET("/health", func(c *core.RequestEvent) error {
		websockets.HealthHandler(c.Response, c.Request)
		return nil
	})

	// Swagger documentation
	e.Router.GET("/swagger/*", func(c *core.RequestEvent) error {
		swaggerHandler := httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"),
		)
		swaggerHandler.ServeHTTP(c.Response, c.Request)
		return nil
	})

	// Serve swagger.json
	e.Router.GET("/swagger/doc.json", func(c *core.RequestEvent) error {
		http.ServeFile(c.Response, c.Request, filepath.Join(baseDir, "docs", "swagger.json"))
		return nil
	})

	// Configurar endpoints para gestionar templates y resource pools
	http_handlers.SetupTemplateRoutes(e, manager)
	http_handlers.SetupResourcePoolRoutes(e, resourcePoolManager, manager)
}

func EnsureDirectoryExists(filePath string) error {
	dir := filepath.Dir(filePath)
	return os.MkdirAll(dir, 0755)
}
