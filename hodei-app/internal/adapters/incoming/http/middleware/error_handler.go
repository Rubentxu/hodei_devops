package middleware

import (
	"encoding/json"
	"log"
	"net/http"
)

// ErrorResponse es la estructura para respuestas de error
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// ErrorHandlerMiddleware maneja los errores de la aplicación
func ErrorHandlerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Utilizar recovery para manejar pánicos
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Error no controlado: %v", err)

				errorResponse := ErrorResponse{
					Error:   "internal_server_error",
					Code:    http.StatusInternalServerError,
					Message: "Se produjo un error interno del servidor",
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(errorResponse)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
