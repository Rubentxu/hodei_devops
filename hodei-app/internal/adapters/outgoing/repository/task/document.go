package task_repository

import "time"

const (
	defaultPageSize = 10
	defaultPage     = 1
	TaskCollection  = "task_definitions"
)

// TaskDocument es la estructura del documento en MongoDB
type TaskDocument struct {
	ID        string     `bson:"id"`
	Metadata  TaskMeta   `bson:"metadata"`
	Spec      TaskSpecDB `bson:"spec"`
	Owner     string     `bson:"owner"`
	TenantID  string     `bson:"tenant_id"`
	CreatedAt time.Time  `bson:"created_at"`
	UpdatedAt time.Time  `bson:"updated_at"`
}

// TaskMeta es la estructura de los metadatos en MongoDB
type TaskMeta struct {
	Name        string            `bson:"name"`
	Description string            `bson:"description,omitempty"`
	Labels      []string          `bson:"labels,omitempty"`
	Annotations map[string]string `bson:"annotations,omitempty"`
	CreatedAt   time.Time         `bson:"createdAt"`
	UpdatedAt   time.Time         `bson:"updatedAt"`
}

// TaskSpecDB es la estructura de la especificación en MongoDB
type TaskSpecDB struct {
	WorkerID    string                 `bson:"worker_id"`
	Command     []string               `bson:"command"`
	Params      []ParamDefinitionDB    `bson:"params"`
	ParamValues map[string]interface{} `bson:"param_values,omitempty"`
}

// ParamDefinitionDB es la estructura de la definición de parámetros en MongoDB
type ParamDefinitionDB struct {
	Key         string             `bson:"key"`
	Type        string             `bson:"type"`
	Label       string             `bson:"label"`
	Description string             `bson:"description,omitempty"`
	Required    bool               `bson:"required"`
	Default     interface{}        `bson:"default,omitempty"`
	Group       string             `bson:"group,omitempty"`
	Order       int                `bson:"order"`
	Validations ParamValidationsDB `bson:"validations,omitempty"`
	Options     []ParamOptionDB    `bson:"options,omitempty"`
	Depends     *ParamDependencyDB `bson:"depends,omitempty"`
}

// ParamValidationsDB es la estructura de validaciones de parámetros en MongoDB
type ParamValidationsDB struct {
	MinLength       *int          `bson:"min_length,omitempty"`
	MaxLength       *int          `bson:"max_length,omitempty"`
	Pattern         string        `bson:"pattern,omitempty"`
	Min             *float64      `bson:"min,omitempty"`
	Max             *float64      `bson:"max,omitempty"`
	Enum            []interface{} `bson:"enum,omitempty"`
	Format          string        `bson:"format,omitempty"`
	CustomValidator string        `bson:"custom_validator,omitempty"`
}

// ParamOptionDB es la estructura de opciones de parámetros en MongoDB
type ParamOptionDB struct {
	Value       interface{} `bson:"value"`
	Label       string      `bson:"label"`
	Description string      `bson:"description,omitempty"`
	Disabled    bool        `bson:"disabled,omitempty"`
}

// ParamDependencyDB es la estructura de dependencias de parámetros en MongoDB
type ParamDependencyDB struct {
	Field    string      `bson:"field"`
	Operator string      `bson:"operator"`
	Value    interface{} `bson:"value"`
}
