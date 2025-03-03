// Package docs Code generated by swaggo/swag. DO NOT EDIT
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "termsOfService": "http://swagger.io/terms/",
        "contact": {
            "name": "API Support",
            "email": "your.email@example.com"
        },
        "license": {
            "name": "MIT",
            "url": "http://opensource.org/licenses/MIT"
        },
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/health": {
            "get": {
                "description": "Retorna el estado de salud del servicio",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "health"
                ],
                "summary": "Endpoint de health check",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/internal_adapters_websockets.HealthResponse"
                        }
                    }
                }
            }
        },
        "/ws": {
            "get": {
                "description": "Endpoint WebSocket para gestionar tareas en tiempo real. Soporta las siguientes acciones:\n- create_task: Crear una nueva tarea\n- stop_task: Detener una tarea en ejecución\n- list_tasks: Listar todas las tareas",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "WebSocket"
                ],
                "summary": "Gestiona conexiones WebSocket para tareas",
                "parameters": [
                    {
                        "type": "string",
                        "description": "ID del cliente para tracking",
                        "name": "client_id",
                        "in": "query"
                    }
                ],
                "responses": {
                    "101": {
                        "description": "Switching Protocols",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/internal_adapters_websockets.ErrorResponse"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "internal_adapters_websockets.ErrorResponse": {
            "type": "object",
            "properties": {
                "code": {
                    "description": "Código de error\nExample: invalid_request",
                    "type": "string"
                },
                "message": {
                    "description": "Mensaje de error\nExample: Invalid request parameters",
                    "type": "string"
                }
            }
        },
        "internal_adapters_websockets.HealthResponse": {
            "type": "object",
            "properties": {
                "status": {
                    "description": "Estado del servicio\nExample: healthy",
                    "type": "string"
                }
            }
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "1.0",
	Host:             "localhost:8080",
	BasePath:         "/",
	Schemes:          []string{"http", "ws"},
	Title:            "Worker API",
	Description:      "API para gestionar tareas y procesos remotos",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
