package generic

import (
	"context"
	"fmt"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// GenericMongoDBWriteRepository implementación genérica de repositorio de escritura
type GenericMongoDBWriteRepository[T model.AggregateRoot, D any] struct {
	collection     *mongo.Collection
	collectionName string
	converter      DocumentConverter[T, D]
}

// NewGenericMongoDBWriteRepository crea una nueva instancia del repositorio genérico
func NewGenericMongoDBWriteRepository[T model.AggregateRoot, D any](
	db *mongo.Database,
	collectionName string,
	converter DocumentConverter[T, D],
) ports.WriteOnlyRepository[T] {
	return &GenericMongoDBWriteRepository[T, D]{
		collection:     db.Collection(collectionName),
		collectionName: collectionName,
		converter:      converter,
	}
}

// Save guarda una entidad y devuelve la entidad con el ID generado
func (r *GenericMongoDBWriteRepository[T, D]) Save(ctx context.Context, entity T) (T, error) {

	doc := r.converter.ToDocument(entity, ctx)
	_, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return entity, fmt.Errorf("%w: %s", ErrDuplicateID, entity.GetID())
		}
		return entity, fmt.Errorf("insert error: %w", err)
	}

	return entity, nil
}

// Update actualiza una entidad existente
func (r *GenericMongoDBWriteRepository[T, D]) Update(ctx context.Context, entity T) error {
	if entity.GetID() == "" {
		return fmt.Errorf("update requires valid ID")
	}

	doc := r.converter.ToDocument(entity, ctx)

	result, err := r.collection.ReplaceOne(ctx, bson.M{"_id": entity.GetID().String()}, doc)
	if err != nil {
		return fmt.Errorf("update error: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("%w: %s", ErrNotFound, entity.GetID())
	}

	return nil
}

// Delete elimina una entidad por su ID
func (r *GenericMongoDBWriteRepository[T, D]) Delete(ctx context.Context, id model.AggregateID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id.String()})
	if err != nil {
		return fmt.Errorf("delete error: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}

	return nil
}

// BatchSave guarda múltiples entidades y devuelve las entidades con IDs generados
func (r *GenericMongoDBWriteRepository[T, D]) BatchSave(ctx context.Context, entities []T) ([]T, error) {
	if len(entities) == 0 {
		return nil, nil
	}

	docs := make([]interface{}, len(entities))
	for i, entity := range entities {
		docs[i] = r.converter.ToDocument(entity, ctx)
	}

	_, err := r.collection.InsertMany(ctx, docs)
	if err != nil {
		return nil, fmt.Errorf("batch insert error: %w", err)
	}

	return entities, nil
}

// BatchUpdate actualiza múltiples entidades
func (r *GenericMongoDBWriteRepository[T, D]) BatchUpdate(ctx context.Context, entities []T) error {
	if len(entities) == 0 {
		return nil
	}

	var models []mongo.WriteModel
	for _, entity := range entities {
		if entity.GetID() == "" {
			return fmt.Errorf("update requires valid ID")
		}
		doc := r.converter.ToDocument(entity, ctx)
		updateModel := mongo.NewReplaceOneModel().
			SetFilter(bson.M{"_id": entity.GetID().String()}).
			SetReplacement(doc)
		models = append(models, updateModel)
	}

	_, err := r.collection.BulkWrite(ctx, models)
	if err != nil {
		return fmt.Errorf("bulk update error: %w", err)
	}

	return nil
}

// BatchDelete elimina múltiples entidades por sus IDs
func (r *GenericMongoDBWriteRepository[T, D]) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	if len(ids) == 0 {
		return nil
	}

	idStrings := make([]string, len(ids))
	for i, id := range ids {
		idStrings[i] = id.String()
	}

	result, err := r.collection.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": idStrings}})
	if err != nil {
		return fmt.Errorf("batch delete error: %w", err)
	}

	if result.DeletedCount != int64(len(ids)) {
		return fmt.Errorf("some resources not found")
	}

	return nil
}
