package rp_repository

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"fmt"
	"time"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	ctxKeyOwner    = "owner"
	ctxKeyTenantID = "tenantID"
)

var _ ports.WriteOnlyRepository[*model.ResourcePoolDef] = (*ResourcePoolMongoDBWriteRepository)(nil)

type ResourcePoolMongoDBWriteRepository struct {
	collection *mongo.Collection
	client     *mongo.Client
	generator  ports.IDGenerator
}

func NewResourcePoolMongoDBWriteRepository(db *mongo.Database, client *mongo.Client, generator ports.IDGenerator) ports.WriteOnlyRepository[*model.ResourcePoolDef] {
	return &ResourcePoolMongoDBWriteRepository{
		collection: db.Collection("resource_pools"),
		client:     client,
		generator:  generator,
	}
}

// modelToDocument convierte la entidad del dominio en un documento para MongoDB.
func (r *ResourcePoolMongoDBWriteRepository) modelToDocument(entity *model.ResourcePoolDef, ctx context.Context) ResourcePoolDocument {
	now := time.Now().UTC()

	if entity.GetID() == "" {
		entity.ID = r.generator.NewID()
	}

	if entity.Metadata.CreatedAt.IsZero() {
		entity.Metadata.CreatedAt = now
	}
	entity.Metadata.UpdatedAt = now

	return ResourcePoolDocument{
		ID: entity.ID.String(),
		Metadata: ResourcePoolMeta{
			Name:        entity.Metadata.Name,
			Description: entity.Metadata.Description,
			Labels:      entity.Metadata.Labels,
			Annotations: entity.Metadata.Annotations,
			CreatedAt:   entity.Metadata.CreatedAt,
			UpdatedAt:   entity.Metadata.UpdatedAt,
		},
		Spec: ResourcePoolSpec{
			PoolID: entity.Spec.PoolID,
			Type:   entity.Spec.Type,
			Config: entity.Spec.ExtendedSpec,
		},
		Status: ResourcePoolStatus{
			State: entity.Status.State,
		},
		Owner:     r.getOwnerFromContext(ctx),
		TenantID:  r.getTenantIDFromContext(ctx),
		CreatedAt: entity.Metadata.CreatedAt,
		UpdatedAt: now,
	}
}

// Save inserta una única entidad y devuelve la entidad con el ID asignado.
func (r *ResourcePoolMongoDBWriteRepository) Save(ctx context.Context, entity *model.ResourcePoolDef) (*model.ResourcePoolDef, error) {

	if entity.GetID() == "" {
		entity.ID = r.generator.NewID()
	}

	doc := r.modelToDocument(entity, ctx)

	_, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, fmt.Errorf("%w: %s", generic.ErrDuplicateID, entity.ID)
		}
		return nil, fmt.Errorf("insert error: %w", err)
	}

	return entity, nil
}

// Update actualiza la entidad existente.
func (r *ResourcePoolMongoDBWriteRepository) Update(ctx context.Context, entity *model.ResourcePoolDef) error {
	if entity.ID == "" {
		return fmt.Errorf("update requires valid ID")
	}

	doc := r.modelToDocument(entity, ctx)
	update := bson.M{"$set": doc}

	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": entity.ID.String()}, update)
	if err != nil {
		return fmt.Errorf("update error: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("%w: %s", generic.ErrNotFound, entity.ID)
	}

	return nil
}

// Delete elimina la entidad indicada.
func (r *ResourcePoolMongoDBWriteRepository) Delete(ctx context.Context, id model.AggregateID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id.String()})
	if err != nil {
		return fmt.Errorf("delete error: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("%w: %s", generic.ErrNotFound, id)
	}

	return nil
}

// BatchSave inserta múltiples entidades y devuelve las entidades con sus IDs asignados.
func (r *ResourcePoolMongoDBWriteRepository) BatchSave(ctx context.Context, entities []*model.ResourcePoolDef) ([]*model.ResourcePoolDef, error) {
	if len(entities) == 0 {
		return nil, nil
	}

	docs := make([]interface{}, len(entities))
	for i, entity := range entities {
		docs[i] = r.modelToDocument(entity, ctx)
	}

	_, err := r.collection.InsertMany(ctx, docs)
	if err != nil {
		return nil, fmt.Errorf("batch insert error: %w", err)
	}

	return entities, nil
}

// BatchUpdate actualiza múltiples entidades.
func (r *ResourcePoolMongoDBWriteRepository) BatchUpdate(ctx context.Context, entities []*model.ResourcePoolDef) error {
	if len(entities) == 0 {
		return nil
	}

	var models []mongo.WriteModel
	for _, entity := range entities {
		if entity.ID == model.AggregateID("") {
			return fmt.Errorf("update requires valid ID")
		}
		doc := r.modelToDocument(entity, ctx)
		updateModel := mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": entity.ID.String()}).
			SetUpdate(bson.M{"$set": doc})
		models = append(models, updateModel)
	}

	_, err := r.collection.BulkWrite(ctx, models)
	if err != nil {
		return fmt.Errorf("bulk update error: %w", err)
	}

	return nil
}

// BatchDelete elimina múltiples entidades.
func (r *ResourcePoolMongoDBWriteRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
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

func (r *ResourcePoolMongoDBWriteRepository) getOwnerFromContext(ctx context.Context) string {
	if owner, ok := ctx.Value(ctxKeyOwner).(string); ok && owner != "" {
		return owner
	}
	return "system"
}

func (r *ResourcePoolMongoDBWriteRepository) getTenantIDFromContext(ctx context.Context) string {
	if tenantID, ok := ctx.Value(ctxKeyTenantID).(string); ok && tenantID != "" {
		return tenantID
	}
	return "default"
}
