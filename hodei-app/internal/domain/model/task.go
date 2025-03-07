package model

import (
	"errors"
)

type Task struct {
	ID       AggregateID
	Metadata Metadata
	TaskSpec TaskSpec
}

type TaskSpec struct {
	WorkerDefinitionID AggregateID            `json:"worker_id" yaml:"worker_id" db:"worker_id"`
	Command            []string               `json:"command" yaml:"command" db:"command"`
	Params             []ParamDefinition      `json:"params" yaml:"params" db:"params"`
	ParamValues        map[string]interface{} `json:"param_values" yaml:"param_values" db:"param_values"`
}

// NewTask crea una nueva tarea con valores predeterminados
func NewTask(name, description string, command []string, params []ParamDefinition) (*Task, error) {
	if name == "" || command == nil || params == nil {
		return nil, errors.New("name, command and params are required")
	}

	return &Task{
		ID: NewAggregateID(),
		Metadata: NewMetadata(
			name,
			description,
		),
		TaskSpec: TaskSpec{
			Command:     command,
			Params:      params,
			ParamValues: make(map[string]interface{}),
		},
	}, nil
}

// ParamDefinition define la estructura de un parámetro en un formulario
type ParamDefinition struct {
	Key         string      `json:"key" yaml:"key" db:"key"`                                 // Identificador único del parámetro
	Type        ParamType   `json:"type" yaml:"type" db:"type"`                              // Tipo de dato del parámetro
	Label       string      `json:"label" yaml:"label" db:"label"`                           // Etiqueta para mostrar en UI
	Description string      `json:"description" yaml:"description" db:"description"`         // Ayuda o descripción
	Required    bool        `json:"required" yaml:"required" db:"required"`                  // Si es obligatorio
	Default     interface{} `json:"default,omitempty" yaml:"default,omitempty" db:"default"` // Valor por defecto
	Group       string      `json:"group,omitempty" yaml:"group,omitempty" db:"group"`       // Grupo lógico para organización
	Order       int         `json:"order" yaml:"order" db:"order"`                           // Orden de visualización

	// Validaciones
	Validations ParamValidations `json:"validations,omitempty" yaml:"validations,omitempty" db:"validations"`

	// Para tipos específicos
	Options []ParamOption    `json:"options,omitempty" yaml:"options,omitempty" db:"options"` // Para select, radio, etc.
	Depends *ParamDependency `json:"depends,omitempty" yaml:"depends,omitempty" db:"depends"` // Para campos que dependen de otros
}

type ParamType string

const (
	ParamTypeString      ParamType = "string"
	ParamTypeInteger     ParamType = "integer"
	ParamTypeNumber      ParamType = "number"
	ParamTypeBoolean     ParamType = "boolean"
	ParamTypeSelect      ParamType = "select"
	ParamTypeMultiSelect ParamType = "multiselect"
	ParamTypeObject      ParamType = "object"
	ParamTypeArray       ParamType = "array"
	ParamTypeDate        ParamType = "date"
	ParamTypeDateTime    ParamType = "datetime"
	ParamTypeFile        ParamType = "file"
	ParamTypePassword    ParamType = "password"
)

type ParamValidations struct {
	MinLength       *int          `json:"minLength,omitempty" yaml:"minLength,omitempty" db:"min_length"`
	MaxLength       *int          `json:"maxLength,omitempty" yaml:"maxLength,omitempty" db:"max_length"`
	Pattern         string        `json:"pattern,omitempty" yaml:"pattern,omitempty" db:"pattern"`
	Min             *float64      `json:"min,omitempty" yaml:"min,omitempty" db:"min"`
	Max             *float64      `json:"max,omitempty" yaml:"max,omitempty" db:"max"`
	Enum            []interface{} `json:"enum,omitempty" yaml:"enum,omitempty" db:"enum"`
	Format          string        `json:"format,omitempty" yaml:"format,omitempty" db:"format"`
	CustomValidator string        `json:"customValidator,omitempty" yaml:"customValidator,omitempty" db:"custom_validator"`
}

type ParamOption struct {
	Value       interface{} `json:"value" yaml:"value" db:"value"`
	Label       string      `json:"label" yaml:"label" db:"label"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty" db:"description"`
	Disabled    bool        `json:"disabled,omitempty" yaml:"disabled,omitempty" db:"disabled"`
}

type ParamDependency struct {
	Field    string      `json:"field" yaml:"field" db:"field"`
	Operator string      `json:"operator" yaml:"operator" db:"operator"` // equals, not_equals, contains, etc.
	Value    interface{} `json:"value" yaml:"value" db:"value"`
}
