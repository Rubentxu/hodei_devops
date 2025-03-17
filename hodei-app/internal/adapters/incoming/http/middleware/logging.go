package middleware

import (
	"log"
	"net/http"
	"time"
)

// LoggingMiddleware registra información sobre cada solicitud HTTP
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("Iniciando solicitud: %s %s", r.Method, r.URL.Path)

		// ResponseWriter personalizado para capturar el código de estado
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		log.Printf("Solicitud completada: %s %s %d %v", r.Method, r.URL.Path, rw.statusCode, duration)
	})
}

// responseWriter es un wrapper de http.ResponseWriter para capturar el código de estado
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captura el código de estado y llama al método original
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
