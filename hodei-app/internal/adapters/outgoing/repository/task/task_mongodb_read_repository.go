package repository

import (
	"context"
	//"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TaskMongoDBReadRepository implementa la interfaz ReadOnlyRepository para Task en MongoDB
var _ ports.ReadOnlyRepository[*model.Task, model.AggregateID] = (*TaskMongoDBReadRepository)(nil)

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

// TaskMongoDBReadRepository implementa operaciones de lectura en MongoDB
type TaskMongoDBReadRepository struct {
	collection *mongo.Collection
}

// NewTaskMongoDBReadRepository crea una nueva instancia del repositorio de lectura
func NewTaskMongoDBReadRepository(db *mongo.Database) ports.ReadOnlyRepository[*model.Task, model.AggregateID] {
	return &TaskMongoDBReadRepository{
		collection: db.Collection("tasks"),
	}
}

// FindByID busca una Task por su ID
func (r *TaskMongoDBReadRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.Task, error) {
	var doc TaskDocument
	err := r.collection.FindOne(ctx, bson.M{"id": id.String()}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("tarea no encontrada con ID: %s", id.String())
		}
		return nil, fmt.Errorf("error al buscar tarea: %w", err)
	}

	return r.documentToModel(&doc)
}

// FindAll retorna todas las Tasks
func (r *TaskMongoDBReadRepository) FindAll(ctx context.Context) ([]*model.Task, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("error al consultar tareas: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []TaskDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("error al decodificar resultados: %w", err)
	}

	result := make([]*model.Task, 0, len(docs))
	for _, doc := range docs {
		task, err := r.documentToModel(&doc)
		if err != nil {
			return nil, err
		}
		result = append(result, task)
	}

	return result, nil
}

// Count devuelve el número total de Tasks
func (r *TaskMongoDBReadRepository) Count(ctx context.Context) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("error al contar tareas: %w", err)
	}
	return count, nil
}

// Exists verifica si existe una Task con el ID proporcionado
func (r *TaskMongoDBReadRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"id": id.String()})
	if err != nil {
		return false, fmt.Errorf("error al verificar existencia: %w", err)
	}
	return count > 0, nil
}

// FindByCriteria busca Tasks aplicando criterios de búsqueda y paginación
func (r *TaskMongoDBReadRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.Task], error) {
	// Construir filtro basado en los criterios
	filter := r.buildFilter(criteria.Filters)

	// Configurar opciones de paginación y ordenamiento
	findOptions := options.Find()
	if criteria.Size > 0 {
		findOptions.SetLimit(int64(criteria.Size))
		findOptions.SetSkip(int64((criteria.Page - 1) * criteria.Size))
	}

	// Configurar ordenamiento
	if criteria.SortBy != "" {
		sortField := r.mapSortField(criteria.SortBy)
		sortOrder := 1 // Ascendente por defecto
		if strings.ToUpper(criteria.SortOrder) == "DESC" {
			sortOrder = -1
		}
		findOptions.SetSort(bson.D{{Key: sortField, Value: sortOrder}})
	} else {
		// Ordenamiento predeterminado por fecha de actualización descendente
		findOptions.SetSort(bson.D{{Key: "updated_at", Value: -1}})
	}

	// Obtener total de elementos que cumplen con el filtro
	totalElements, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return ports.SearchResult[*model.Task]{}, fmt.Errorf("error al contar elementos filtrados: %w", err)
	}

	// Ejecutar consulta con paginación
	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return ports.SearchResult[*model.Task]{}, fmt.Errorf("error al buscar con criterios: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []TaskDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return ports.SearchResult[*model.Task]{}, fmt.Errorf("error al decodificar resultados: %w", err)
	}

	// Convertir documentos a modelos de dominio
	content := make([]*model.Task, 0, len(docs))
	for _, doc := range docs {
		task, err := r.documentToModel(&doc)
		if err != nil {
			return ports.SearchResult[*model.Task]{}, err
		}
		content = append(content, task)
	}

	// Calcular información de paginación
	pageSize := criteria.Size
	if pageSize <= 0 {
		pageSize = 10 // valor por defecto
	}

	totalPages := int(totalElements / int64(pageSize))
	if totalElements%int64(pageSize) > 0 {
		totalPages++
	}

	currentPage := criteria.Page
	if currentPage <= 0 {
		currentPage = 1
	}

	return ports.SearchResult[*model.Task]{
		Content:       content,
		TotalElements: totalElements,
		TotalPages:    totalPages,
		Page:          currentPage,
		Size:          pageSize,
		HasNext:       currentPage < totalPages,
		HasPrevious:   currentPage > 1,
	}, nil
}

// Métodos auxiliares

// buildFilter construye un filtro BSON a partir de los criterios de búsqueda
func (r *TaskMongoDBReadRepository) buildFilter(filters map[string]interface{}) bson.M {
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
			filter["metadata.name"] = bson.M{"$regex": primitive.Regex{Pattern: value.(string), Options: "i"}}
		case "descriptionContains":
			filter["metadata.description"] = bson.M{"$regex": primitive.Regex{Pattern: value.(string), Options: "i"}}
		case "commandContains":
			if strValue, ok := value.(string); ok {
				filter["spec.command"] = bson.M{"$regex": primitive.Regex{Pattern: strValue, Options: "i"}}
			}
		}
	}

	return filter
}

// mapSortField mapea el nombre de campo para ordenamiento
func (r *TaskMongoDBReadRepository) mapSortField(sortBy string) string {
	switch sortBy {
	case "name":
		return "metadata.name"
	case "workerID":
		return "spec.worker_id"
	case "createdAt":
		return "created_at"
	case "updatedAt":
		return "updated_at"
	default:
		return sortBy
	}
}

// documentToModel convierte un documento de MongoDB a un modelo de dominio Task
func (r *TaskMongoDBReadRepository) documentToModel(doc *TaskDocument) (*model.Task, error) {
	id, err := uuid.Parse(doc.ID)
	if err != nil {
		return nil, fmt.Errorf("error al parsear ID: %w", err)
	}

	// Parsear WorkerID
	workerID, err := uuid.Parse(doc.Spec.WorkerID)
	if err != nil {
		return nil, fmt.Errorf("error al parsear WorkerID: %w", err)
	}

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
			WorkerDefinitionID: model.AggregateID(workerID),
			Command:            doc.Spec.Command,
			Params:             params,
			ParamValues:        doc.Spec.ParamValues,
		},
	}

	return task, nil
}
