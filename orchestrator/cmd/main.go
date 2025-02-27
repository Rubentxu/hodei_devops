package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	httpSwagger "github.com/swaggo/http-swagger"

	"dev.rubentxu.devops-platform/orchestrator/config"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/manager"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/resources"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/websockets"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/worker"
	"dev.rubentxu.devops-platform/orchestrator/internal/adapters/worker/factories"

	_ "dev.rubentxu.devops-platform/orchestrator/docs"
)

func main() {
	// Crear nueva instancia de PocketBase
	app := pocketbase.New()

	// Cargar configuración
	cfg := config.Load()
	log.Printf("Configuración cargada en worker api: %+v", cfg)

	// Inicializar componentes del worker
	workerFactory := factories.NewWorkerInstanceFactory(cfg)
	w := worker.NewWorker(
		cfg.WorkerName,
		cfg.MaxConcurrentTasks,
		workerFactory,
	)

	// Crear manager
	manager, err := manager.New("greedy", cfg.StorageType, w, 100, app)
	if err != nil {
		log.Fatalf("Error creando el manager: %v", err)
	}

	// Crear pool de recursos Docker
	dockerResourcePool, err := resources.NewDockerResourcePool(cfg.Providers.Docker, "defaultDockerPool")
	if err != nil {
		log.Fatalf("Error creando el pool de recursos Docker: %v", err)
	}

	// Inicializar WebSocket handler
	wsHandler := websockets.NewWSHandler(manager)

	// Añadir pool de recursos y comenzar procesamiento de tareas
	manager.AddResourcePool(dockerResourcePool)
	go manager.ProcessTasks()

	// Configurar rutas en PocketBase
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Obtener directorio base
		baseDir, err := filepath.Abs(".")
		if err != nil {
			return fmt.Errorf("error obteniendo directorio base: %w", err)
		}

		// Configurar rutas estáticas
		e.Router.GET("/static/{path...}", apis.Static(os.DirFS("./pb_public"), false))

		// Configurar endpoints del worker
		setupWorkerRoutes(e, wsHandler, baseDir)

		return e.Next()
	})

	// Determinar la dirección de escucha - usar 0.0.0.0 para permitir conexiones externas
	bindAddr := os.Getenv("PB_ADDR")
	if bindAddr == "" {
		bindAddr = "0.0.0.0:8090"
	}

	// Start server
	log.Printf("Server starting at %s", bindAddr)
	log.Printf("├─ REST API:  http://%s/api/", bindAddr)
	log.Printf("└─ Dashboard: http://%s/_/", bindAddr)

	// Check if PB_SKIP_INIT is set; if not, print superuser creation instructions
	if os.Getenv("PB_SKIP_INIT") != "true" {
		log.Printf("(!) Launch the URL below in the browser if it hasn't been open already to create your first superuser account:")
		log.Printf("http://localhost:8090/_/#/pbinstal/%%s", "<TOKEN>")
		log.Printf("(you can also create your first superuser by running: /app/orchestrator superuser upsert EMAIL PASS)")
	}

	// Configure the server via environment
	if err := os.Setenv("SERVE_HTTP", bindAddr); err != nil {
		log.Fatal(err)
	}

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func setupWorkerRoutes(e *core.ServeEvent, wsHandler *websockets.WSHandler, baseDir string) {
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
}
