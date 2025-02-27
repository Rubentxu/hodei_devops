package utils

import (
	"log"
	"os"

	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/manager"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/resources"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/websockets"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// Start inicia el servidor HTTP con todas las configuraciones necesarias
func Start(
	app *pocketbase.PocketBase,
	wsHandler *websockets.WSHandler,
	manager *manager.Manager,
	resourcePoolManager *resources.ResourcePoolManager,
) {
	// Determinar la dirección de escucha - usar 0.0.0.0 para permitir conexiones externas
	bindAddr := os.Getenv("PB_ADDR")
	if bindAddr == "" {
		bindAddr = "0.0.0.0:8090"
	}

	// Configurar rutas en PocketBase
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Configurar rutas estáticas
		e.Router.GET("/static/{path...}", apis.Static(os.DirFS("./pb_public"), false))

		// Configurar endpoints del worker
		SetupRoutes(e, wsHandler, manager, resourcePoolManager)

		return e.Next()
	})

	// Configure the server via environment
	if err := os.Setenv("SERVE_HTTP", bindAddr); err != nil {
		log.Fatal(err)
	}

	// Mostrar información de inicio
	log.Printf("╔════════════════════════════════════════════╗")
	log.Printf("║  Servidor iniciando en %s           ║", bindAddr)
	log.Printf("║────────────────────────────────────────────║")
	log.Printf("║  REST API:  http://%s/api/             ║", bindAddr)
	log.Printf("║  Dashboard: http://%s/_/              ║", bindAddr)
	log.Printf("║  WebSocket: ws://%s/ws               ║", bindAddr)
	log.Printf("╚════════════════════════════════════════════╝")

	// Iniciar el servidor
	if err := app.Start(); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
