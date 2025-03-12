package model

import (
	"errors"
)

type Task struct {
	ID       AggregateID `json:"id" validate:"required"`
	Metadata Metadata    `json:"metadata" validate:"required"`
	Spec     TaskSpec    `json:"spec" validate:"required"`
}

func (t Task) GetID() AggregateID {
	return t.ID
}

type TaskSpec struct {
	WorkerDefinitionID AggregateID            `json:"worker_id" validate:"required"`
	Command            []string               `json:"command" validate:"required,min=1"`
	Params             []ParamDefinition      `json:"params" validate:"dive"`
	ParamValues        map[string]interface{} `json:"param_values"`
}

// NewTask crea una nueva tarea con valores predeterminados
func NewTask(name, description string, command []string, params []ParamDefinition) (*Task, error) {
	if name == "" || command == nil || params == nil {
		return nil, errors.New("name, command and params are required")
	}

	return &Task{
		Metadata: NewMetadata(
			name,
			description,
		),
		Spec: TaskSpec{
			Command:     command,
			Params:      params,
			ParamValues: make(map[string]interface{}),
		},
	}, nil
}

type ParamDefinition struct {
	Key         string           `json:"key" validate:"required"`
	Type        ParamType        `json:"type" validate:"required,oneof=string integer number boolean select multiselect object array date datetime file password"`
	Label       string           `json:"label" validate:"required"`
	Description string           `json:"description"`
	Required    bool             `json:"required"`
	Default     interface{}      `json:"default,omitempty"`
	Group       string           `json:"group,omitempty"`
	Order       int              `json:"order" validate:"gte=0"`
	Validations ParamValidations `json:"validations,omitempty" validate:"omitempty"`
	Options     []ParamOption    `json:"options,omitempty" validate:"omitempty,dive"`
	Depends     *ParamDependency `json:"depends,omitempty" validate:"omitempty"`
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
	MinLength       *int          `json:"minLength,omitempty" validate:"omitempty,gte=0"`
	MaxLength       *int          `json:"maxLength,omitempty" validate:"omitempty,gtefield=MinLength"`
	Pattern         string        `json:"pattern,omitempty" validate:"omitempty"`
	Min             *float64      `json:"min,omitempty" validate:"omitempty"`
	Max             *float64      `json:"max,omitempty" validate:"omitempty,gtefield=Min"`
	Enum            []interface{} `json:"enum,omitempty" validate:"omitempty,min=1"`
	Format          string        `json:"format,omitempty" validate:"omitempty"`
	CustomValidator string        `json:"customValidator,omitempty" validate:"omitempty"`
}

type ParamOption struct {
	Value       interface{} `json:"value" validate:"required"`
	Label       string      `json:"label" validate:"required"`
	Description string      `json:"description,omitempty"`
	Disabled    bool        `json:"disabled,omitempty"`
}

type ParamDependency struct {
	Field    string      `json:"field" validate:"required"`
	Operator string      `json:"operator" validate:"required,oneof=equals not_equals contains"`
	Value    interface{} `json:"value" validate:"required"`
}
