package generic

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"strings"
)

const (
	defaultPageSize = 10
	defaultPage     = 1
)

var (
	ErrDuplicateID                                               = errors.New("duplicate resource pool ID")
	ErrNotFound                                                  = errors.New("resource not found")
	_              ports.ReadOnlyRepository[model.AggregateRoot] = (*GenericMongoDBReadRepository[model.AggregateRoot, any])(nil)
)

type DocumentConverter[T model.AggregateRoot, D any] interface {
	ToModel(doc D) (T, error)
	ToDocument(entity T, ctx context.Context) D
	BuildFilter(filters map[string]interface{}) bson.M
	MapSortField(field string) string
}

// GenericMongoDBReadRepository implementación genérica de repositorio de lectura
type GenericMongoDBReadRepository[T model.AggregateRoot, D any] struct {
	collection     *mongo.Collection
	collectionName string
	converter      DocumentConverter[T, D]
}

func NewGenericMongoDBReadRepository[T model.AggregateRoot, D any](
	db *mongo.Database,
	collectionName string,
	converter DocumentConverter[T, D],
) ports.ReadOnlyRepository[T] {
	return &GenericMongoDBReadRepository[T, D]{
		collection:     db.Collection(collectionName),
		collectionName: collectionName,
		converter:      converter,
	}
}

// FindByID busca una entidad por su ID
func (r *GenericMongoDBReadRepository[T, D]) FindByID(ctx context.Context, id model.AggregateID) (T, error) {
	var doc D
	var zero T

	// Usamos id.String() para convertir el ID al formato que espera MongoDB
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return zero, fmt.Errorf("%s not found: %w", r.collectionName, ErrNotFound)
		}
		return zero, fmt.Errorf("find by ID error: %w", err)
	}

	return r.converter.ToModel(doc)
}

// FindAll recupera todas las entidades
func (r *GenericMongoDBReadRepository[T, D]) FindAll(ctx context.Context) ([]T, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("find all error: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []D
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	return r.convertDocuments(docs)
}

// Count cuenta el número total de entidades
func (r *GenericMongoDBReadRepository[T, D]) Count(ctx context.Context) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("count error: %w", err)
	}
	return count, nil
}

// Exists verifica si existe una entidad con el ID proporcionado
func (r *GenericMongoDBReadRepository[T, D]) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"_id": id})
	if err != nil {
		return false, fmt.Errorf("exists check error: %w", err)
	}
	return count > 0, nil
}

// FindByCriteria busca entidades aplicando criterios y paginación
func (r *GenericMongoDBReadRepository[T, D]) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[T], error) {
	filter := r.converter.BuildFilter(criteria.Filters)
	findOptions := r.buildFindOptions(criteria)

	totalElements, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return ports.SearchResult[T]{}, fmt.Errorf("count error: %w", err)
	}

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return ports.SearchResult[T]{}, fmt.Errorf("find error: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []D
	if err := cursor.All(ctx, &docs); err != nil {
		return ports.SearchResult[T]{}, fmt.Errorf("decode error: %w", err)
	}

	content, err := r.convertDocuments(docs)
	if err != nil {
		return ports.SearchResult[T]{}, err
	}

	pageSize := criteria.Size
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	page := criteria.Page
	if page <= 0 {
		page = defaultPage
	}

	totalPages := int(totalElements / int64(pageSize))
	if totalElements%int64(pageSize) != 0 {
		totalPages++
	}

	hasNext := page < totalPages

	return ports.SearchResult[T]{
		Content:       content,
		TotalElements: totalElements,
		TotalPages:    totalPages,
		Page:          page,
		Size:          pageSize,
		HasNext:       hasNext,
		HasPrevious:   page > 1,
	}, nil
}

// buildFindOptions construye las opciones para consultas paginadas
func (r *GenericMongoDBReadRepository[T, D]) buildFindOptions(criteria ports.SearchCriteria) *options.FindOptions {
	findOptions := options.Find()

	// Aplicar paginación
	page := criteria.Page
	if page <= 0 {
		page = defaultPage
	}

	size := criteria.Size
	if size <= 0 {
		size = defaultPageSize
	}

	findOptions.SetLimit(int64(size))
	findOptions.SetSkip(int64((page - 1) * size))

	// Aplicar ordenación
	sortField := r.converter.MapSortField(criteria.SortBy)
	sortOrder := 1
	if strings.ToUpper(criteria.SortOrder) == "DESC" {
		sortOrder = -1
	}
	findOptions.SetSort(bson.D{{Key: sortField, Value: sortOrder}})

	return findOptions
}

// convertDocuments convierte una lista de documentos en una lista de modelos
func (r *GenericMongoDBReadRepository[T, D]) convertDocuments(docs []D) ([]T, error) {
	result := make([]T, 0, len(docs))
	for _, doc := range docs {
		model, err := r.converter.ToModel(doc)
		if err != nil {
			return nil, err
		}
		result = append(result, model)
	}
	return result, nil
}
