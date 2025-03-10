package repository

import (
	"context"
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

// ResourcePoolMongoDBReadRepository implementa la interfaz ReadOnlyRepository para ResourcePoolDef en MongoDB
var _ ports.ReadOnlyRepository[*model.ResourcePoolDef, model.AggregateID] = (*ResourcePoolMongoDBReadRepository)(nil)

// ResourcePoolDocument es la estructura del documento en MongoDB
type ResourcePoolDocument struct {
	ID       string             `bson:"id"`
	Metadata ResourcePoolMeta   `bson:"metadata"`
	Spec     ResourcePoolSpecDB `bson:"spec"`
	Status   ResourcePoolStatus `bson:"status"`
	Owner    string             `bson:"owner"`
	TenantID string             `bson:"tenant_id"`
	CreatedAt time.Time         `bson:"created_at"`
	UpdatedAt time.Time         `bson:"updated_at"`
}

// ResourcePoolMeta es la estructura de los metadatos en MongoDB
type ResourcePoolMeta struct {
	Name        string            `bson:"name"`
	Description string            `bson:"description,omitempty"`
	Labels      []string          `bson:"labels,omitempty"`
	Annotations map[string]string `bson:"annotations,omitempty"`
	CreatedAt   time.Time         `bson:"createdAt"`
	UpdatedAt   time.Time         `bson:"updatedAt"`
}

// ResourcePoolSpecDB es la estructura de la especificación en MongoDB
type ResourcePoolSpecDB struct {
	PoolID string                 `bson:"poolID"`
	Type   string                 `bson:"type"`
	Config map[string]interface{} `bson:"config,omitempty"`
}

// ResourcePoolStatus es la estructura del estado en MongoDB
type ResourcePoolStatus struct {
	State string `bson:"state"`
}

// ResourcePoolMongoDBReadRepository implementa operaciones de lectura en MongoDB
type ResourcePoolMongoDBReadRepository struct {
	collection *mongo.Collection
}

// NewResourcePoolMongoDBReadRepository crea una nueva instancia del repositorio de lectura
func NewResourcePoolMongoDBReadRepository(db *mongo.Database) ports.ReadOnlyRepository[*model.ResourcePoolDef, model.AggregateID] {
	return &ResourcePoolMongoDBReadRepository{
		collection: db.Collection("resource_pools"),
	}
}

// FindByID busca un ResourcePoolDef por su ID
func (r *ResourcePoolMongoDBReadRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.ResourcePoolDef, error) {
	var doc ResourcePoolDocument
	err := r.collection.FindOne(ctx, bson.M{"id": id.String()}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("resource pool no encontrado con ID: %s", id.String())
		}
		return nil, fmt.Errorf("error al buscar resource pool: %w", err)
	}

	return r.documentToModel(&doc)
}

// FindAll retorna todos los ResourcePoolDef
func (r *ResourcePoolMongoDBReadRepository) FindAll(ctx context.Context) ([]*model.ResourcePoolDef, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("error al consultar resource pools: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []ResourcePoolDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("error al decodificar resultados: %w", err)
	}

	result := make([]*model.ResourcePoolDef, 0, len(docs))
	for _, doc := range docs {
		pool, err := r.documentToModel(&doc)
		if err != nil {
			return nil, err
		}
		result = append(result, pool)
	}

	return result, nil
}

// Count devuelve el número total de ResourcePoolDef
func (r *ResourcePoolMongoDBReadRepository) Count(ctx context.Context) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("error al contar resource pools: %w", err)
	}
	return count, nil
}

// Exists verifica si existe un ResourcePoolDef con el ID proporcionado
func (r *ResourcePoolMongoDBReadRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"id": id.String()})
	if err != nil {
		return false, fmt.Errorf("error al verificar existencia: %w", err)
	}
	return count > 0, nil
}

// FindByCriteria busca ResourcePoolDef aplicando criterios de búsqueda y paginación
func (r *ResourcePoolMongoDBReadRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.ResourcePoolDef], error) {
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
		return ports.SearchResult[*model.ResourcePoolDef]{}, fmt.Errorf("error al contar elementos filtrados: %w", err)
	}

	// Ejecutar consulta con paginación
	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return ports.SearchResult[*model.ResourcePoolDef]{}, fmt.Errorf("error al buscar con criterios: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []ResourcePoolDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return ports.SearchResult[*model.ResourcePoolDef]{}, fmt.Errorf("error al decodificar resultados: %w", err)
	}

	// Convertir documentos a modelos de dominio
	content := make([]*model.ResourcePoolDef, 0, len(docs))
	for _, doc := range docs {
		pool, err := r.documentToModel(&doc)
		if err != nil {
			return ports.SearchResult[*model.ResourcePoolDef]{}, err
		}
		content = append(content, pool)
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

	return ports.SearchResult[*model.ResourcePoolDef]{
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
func (r *ResourcePoolMongoDBReadRepository) buildFilter(filters map[string]interface{}) bson.M {
	if filters == nil || len(filters) == 0 {
		return bson.M{}
	}

	filter := bson.M{}

	for key, value := range filters {
		switch key {
		case "type":
			filter["spec.type"] = value
		case "name":
			filter["metadata.name"] = value
		case "poolID":
			filter["spec.poolID"] = value
		case "state":
			filter["status.state"] = value
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
		}
	}

	return filter
}

// mapSortField mapea el nombre de campo para ordenamiento
func (r *ResourcePoolMongoDBReadRepository) mapSortField(sortBy string) string {
	switch sortBy {
	case "name":
		return "metadata.name"
	case "type":
		return "spec.type"
	case "poolID":
		return "spec.poolID"
	case "state":
		return "status.state"
	case "createdAt":
		return "created_at"
	case "updatedAt":
		return "updated_at"
	default:
		return sortBy
	}
}

// documentToModel convierte un documento de MongoDB a un modelo de dominio
func (r *ResourcePoolMongoDBReadRepository) documentToModel(doc *ResourcePoolDocument) (*model.ResourcePoolDef, error) {
	id, err := uuid.Parse(doc.ID)
	if err != nil {
		return nil, fmt.Errorf("error al parsear ID: %w", err)
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

	// Mapear spec
	spec := model.ResourcePoolSpec{
		PoolID:       doc.Spec.PoolID,
		Type:         doc.Spec.Type,
		ExtendedSpec: doc.Spec.Config,
	}

	// Mapear status
	status := model.ResourcePoolStatus{
		State: doc.Status.State,
	}

	// Construir el modelo de dominio
	return &model.ResourcePoolDef{
		ID:       model.AggregateID(id),
		Metadata: metadata,
		Spec:     spec,
		Status:   status,
	}, nil
}