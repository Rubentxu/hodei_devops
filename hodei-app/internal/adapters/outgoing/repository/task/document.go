package task_repository

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"regexp"
	"time"
)

const (
	TaskCollection = "task_definitions"
)

// TaskDocument es la estructura del documento en MongoDB
type TaskDocument struct {
	ID        string     `bson:"_id"`
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

// TaskDocumentConverter implementa la interfaz DocumentConverter para Task
type TaskDocumentConverter struct {
	generator ports.IDGenerator
}

func NewTaskDocumentConverter(generator ports.IDGenerator) generic.DocumentConverter[*model.Task, TaskDocument] {
	return &TaskDocumentConverter{
		generator: generator,
	}
}

func (c *TaskDocumentConverter) GenerateID() model.AggregateID {
	return c.generator.NewID()
}

// ToModel convierte un documento de MongoDB a un modelo de dominio Task
func (c *TaskDocumentConverter) ToModel(doc TaskDocument) (*model.Task, error) {
	id := doc.ID

	// Mapear metadatos
	metadata := model.Metadata{
		Name:        doc.Metadata.Name,
		Description: doc.Metadata.Description,
		Labels:      doc.Metadata.Labels,
		Annotations: doc.Metadata.Annotations,
		CreatedAt:   doc.Metadata.CreatedAt,
		UpdatedAt:   doc.Metadata.UpdatedAt,
	}

	// Convertir parámetros de documento a modelo
	params := make([]model.ParamDefinition, len(doc.Spec.Params))
	for i, param := range doc.Spec.Params {
		paramType := model.ParamType(param.Type)

		var options []model.ParamOption
		for _, opt := range param.Options {
			options = append(options, model.ParamOption{
				Value:       opt.Value,
				Label:       opt.Label,
				Description: opt.Description,
				Disabled:    opt.Disabled,
			})
		}

		var depends *model.ParamDependency
		if param.Depends != nil {
			depends = &model.ParamDependency{
				Field:    param.Depends.Field,
				Operator: param.Depends.Operator,
				Value:    param.Depends.Value,
			}
		}

		validations := model.ParamValidations{
			MinLength:       param.Validations.MinLength,
			MaxLength:       param.Validations.MaxLength,
			Pattern:         param.Validations.Pattern,
			Min:             param.Validations.Min,
			Max:             param.Validations.Max,
			Enum:            param.Validations.Enum,
			Format:          param.Validations.Format,
			CustomValidator: param.Validations.CustomValidator,
		}

		params[i] = model.ParamDefinition{
			Key:         param.Key,
			Type:        paramType,
			Label:       param.Label,
			Description: param.Description,
			Required:    param.Required,
			Default:     param.Default,
			Group:       param.Group,
			Order:       param.Order,
			Validations: validations,
			Options:     options,
			Depends:     depends,
		}
	}

	// Construir el modelo de dominio
	task := &model.Task{
		ID:       model.AggregateID(id),
		Metadata: metadata,
		Spec: model.TaskSpec{
			WorkerDefinitionID: model.AggregateID(doc.Spec.WorkerID),
			Command:            doc.Spec.Command,
			Params:             params,
			ParamValues:        doc.Spec.ParamValues,
		},
	}

	return task, nil
}

// ToDocument convierte un modelo de dominio Task a un documento de MongoDB
func (c *TaskDocumentConverter) ToDocument(entity *model.Task, ctx context.Context) TaskDocument {
	if entity.ID == "" {
		entity.ID = c.GenerateID()
	}
	return TaskDocument{
		ID: entity.ID.String(),
		Metadata: TaskMeta{
			Name:        entity.Metadata.Name,
			Description: entity.Metadata.Description,
			Labels:      entity.Metadata.Labels,
			Annotations: entity.Metadata.Annotations,
			CreatedAt:   entity.Metadata.CreatedAt,
			UpdatedAt:   entity.Metadata.UpdatedAt,
		},
		Spec: TaskSpecDB{
			WorkerID:    entity.Spec.WorkerDefinitionID.String(),
			Command:     entity.Spec.Command,
			Params:      convertParamsToDocuments(entity.Spec.Params),
			ParamValues: entity.Spec.ParamValues,
		},
		CreatedAt: entity.Metadata.CreatedAt,
		UpdatedAt: entity.Metadata.UpdatedAt,
	}
}

// BuildFilter construye un filtro BSON a partir de los criterios de búsqueda
func (c *TaskDocumentConverter) BuildFilter(filters map[string]interface{}) bson.M {
	if filters == nil || len(filters) == 0 {
		return bson.M{}
	}

	filter := bson.M{}

	for key, value := range filters {
		switch key {
		case "name":
			filter["metadata.name"] = value
		case "workerID":
			filter["spec.worker_id"] = value
		case "owner":
			filter["owner"] = value
		case "tenantId":
			filter["tenant_id"] = value
		case "labels":
			if labels, ok := value.([]string); ok && len(labels) > 0 {
				filter["metadata.labels"] = bson.M{"$all": labels}
			}
		case "nameContains":
			if strValue, ok := value.(string); ok {
				filter["metadata.name"] = bson.M{"$regex": primitive.Regex{
					Pattern: regexp.QuoteMeta(strValue),
					Options: "i",
				}}
			}
		case "descriptionContains":
			if strValue, ok := value.(string); ok {
				filter["metadata.description"] = bson.M{"$regex": primitive.Regex{
					Pattern: regexp.QuoteMeta(strValue),
					Options: "i",
				}}
			}
		case "commandContains":
			if strValue, ok := value.(string); ok {
				filter["spec.command"] = bson.M{"$regex": primitive.Regex{
					Pattern: regexp.QuoteMeta(strValue),
					Options: "i",
				}}
			}
		}
	}

	return filter
}

// MapSortField mapea el nombre de campo para ordenamiento
func (c *TaskDocumentConverter) MapSortField(field string) string {
	switch field {
	case "name":
		return "metadata.name"
	case "workerID":
		return "spec.worker_id"
	case "createdAt":
		return "created_at"
	case "updatedAt":
		return "updated_at"
	default:
		return "_id"
	}
}

// Función auxiliar para convertir parámetros de modelo a documento
func convertParamsToDocuments(params []model.ParamDefinition) []ParamDefinitionDB {
	result := make([]ParamDefinitionDB, len(params))
	for i, param := range params {
		options := make([]ParamOptionDB, len(param.Options))
		for j, opt := range param.Options {
			options[j] = ParamOptionDB{
				Value:       opt.Value,
				Label:       opt.Label,
				Description: opt.Description,
				Disabled:    opt.Disabled,
			}
		}

		var depends *ParamDependencyDB
		if param.Depends != nil {
			depends = &ParamDependencyDB{
				Field:    param.Depends.Field,
				Operator: param.Depends.Operator,
				Value:    param.Depends.Value,
			}
		}

		result[i] = ParamDefinitionDB{
			Key:         param.Key,
			Type:        string(param.Type),
			Label:       param.Label,
			Description: param.Description,
			Required:    param.Required,
			Default:     param.Default,
			Group:       param.Group,
			Order:       param.Order,
			Validations: ParamValidationsDB{
				MinLength:       param.Validations.MinLength,
				MaxLength:       param.Validations.MaxLength,
				Pattern:         param.Validations.Pattern,
				Min:             param.Validations.Min,
				Max:             param.Validations.Max,
				Enum:            param.Validations.Enum,
				Format:          param.Validations.Format,
				CustomValidator: param.Validations.CustomValidator,
			},
			Options: options,
			Depends: depends,
		}
	}
	return result
}
