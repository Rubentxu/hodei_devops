// Package apidocs Worker API Documentation
//
// # Documentación de la API para gestionar tareas y procesos remotos
//
// Terms Of Service:
//
//	Schemes: http, ws
//	Host: localhost:8080
//	BasePath: /
//	Version: 1.0.0
//	License: MIT http://opensource.org/licenses/MIT
//
//	Consumes:
//	- application/json
//
//	Produces:
//	- application/json
//
//	Security:
//	- basic
//
//	SecurityDefinitions:
//	basic:
//	  type: basic
//
// swagger:meta
package apidocs

import (
	"dev.rubentxu.devops-platform/orchestrator/internal/domain"
)

// swagger:response taskResponse
type taskResponse struct {
	// in: body
	Body domain.Task
}

// swagger:parameters createTask
type taskParams struct {
	// in: body
	Body struct {
		Name         string            `json:"name"`
		Image        string            `json:"image"`
		Command      []string          `json:"command,omitempty"`
		Env          map[string]string `json:"env,omitempty"`
		WorkingDir   string            `json:"working_dir,omitempty"`
		InstanceType string            `json:"instance_type,omitempty"`
		Timeout      int               `json:"timeout,omitempty"`
	}
}

// swagger:parameters wsConnect
type wsConnectParams struct {
	// ID del cliente para tracking
	// in: query
	ClientID string `json:"client_id"`
}

// swagger:response errorResponse
type errorResponse struct {
	// in: body
	Body struct {
		// Código de error
		// required: true
		Code string `json:"code"`

		// Mensaje de error
		// required: true
		Message string `json:"message"`
	}
}

// swagger:response healthResponse
type healthResponse struct {
	// in: body
	Body struct {
		// Estado del servicio
		// required: true
		// example: healthy
		Status string `json:"status"`
	}
}

// swagger:response wsMessageResponse
type wsMessageResponse struct {
	// in: body
	Body struct {
		// Tipo de mensaje
		// required: true
		Type string `json:"type"`

		// Contenido del mensaje
		// required: true
		Data interface{} `json:"data"`
	}
}
