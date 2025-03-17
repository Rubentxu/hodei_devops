package http

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"
)

// ServerConfig contiene la configuración del servidor HTTP
type ServerConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DefaultServerConfig retorna una configuración por defecto
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr:         ":8080",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// Server encapsula el servidor HTTP
type Server struct {
	server *http.Server
	router *mux.Router
}

// NewServer crea una nueva instancia del servidor HTTP
func NewServer(
	config ServerConfig,
	resourcePoolService ports.ResourcePoolService,
	taskExecutionService ports.TaskExecutionService,
) *Server {
	router := SetupRouter(resourcePoolService, taskExecutionService)

	srv := &http.Server{
		Addr:         config.Addr,
		Handler:      router,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return &Server{
		server: srv,
		router: router,
	}
}

// GetRouter retorna el router utilizado por el servidor
func (s *Server) GetRouter() *mux.Router {
	return s.router
}

// Start inicia el servidor HTTP
func (s *Server) Start() error {
	log.Printf("Iniciando servidor HTTP en %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// StartAsync inicia el servidor HTTP en una goroutine
func (s *Server) StartAsync() {
	go func() {
		if err := s.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error al iniciar servidor: %v", err)
		}
	}()
}

// Shutdown detiene graciosamente el servidor HTTP
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Cerrando servidor HTTP...")
	return s.server.Shutdown(ctx)
}
