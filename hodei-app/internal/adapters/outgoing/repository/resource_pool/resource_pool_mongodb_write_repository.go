package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	ctxKeyOwner    = "owner"
	ctxKeyTenantID = "tenantID"
)

var (
	ErrDuplicateID = errors.New("duplicate resource pool ID")
	ErrNotFound    = errors.New("resource pool not found")
)

type ResourcePoolMongoDBWriteRepository struct {
	collection *mongo.Collection
	client     *mongo.Client
}

func NewResourcePoolMongoDBWriteRepository(db *mongo.Database, client *mongo.Client) ports.WriteOnlyRepository[*model.ResourcePoolDef, model.AggregateID] {
	return &ResourcePoolMongoDBWriteRepository{
		collection: db.Collection("resource_pools"),
		client:     client,
	}
}

type ResourcePoolDocument struct {
	ID        string             `bson:"_id"`
	Metadata  ResourcePoolMeta   `bson:"metadata"`
	Spec      ResourcePoolSpec   `bson:"spec"`
	Status    ResourcePoolStatus `bson:"status"`
	Owner     string             `bson:"owner"`
	TenantID  string             `bson:"tenant_id"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

type ResourcePoolMeta struct {
	Name        string            `bson:"name"`
	Description string            `bson:"description"`
	Labels      []string          `bson:"labels"`
	Annotations map[string]string `bson:"annotations"`
	CreatedAt   time.Time         `bson:"created_at"`
	UpdatedAt   time.Time         `bson:"updated_at"`
}

type ResourcePoolSpec struct {
	PoolID string                 `bson:"pool_id"`
	Type   string                 `bson:"type"`
	Config map[string]interface{} `bson:"config"`
}

type ResourcePoolStatus struct {
	State string `bson:"state"`
}

func (r *ResourcePoolMongoDBWriteRepository) modelToDocument(entity *model.ResourcePoolDef, ctx context.Context) ResourcePoolDocument {
	now := time.Now().UTC()

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

func (r *ResourcePoolMongoDBWriteRepository) Save(ctx context.Context, entity *model.ResourcePoolDef) error {
	if entity.ID == model.AggregateID(uuid.Nil) {
		entity.ID = model.NewAggregateID()
	}

	doc := r.modelToDocument(entity, ctx)

	_, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("%w: %s", ErrDuplicateID, entity.ID)
		}
		return fmt.Errorf("insert error: %w", err)
	}

	return nil
}

func (r *ResourcePoolMongoDBWriteRepository) Update(ctx context.Context, entity *model.ResourcePoolDef) error {
	if entity.ID == model.AggregateID(uuid.Nil) {
		return fmt.Errorf("update requires valid ID")
	}

	doc := r.modelToDocument(entity, ctx)
	update := bson.M{"$set": doc}

	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": entity.ID.String()}, update)
	if err != nil {
		return fmt.Errorf("update error: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("%w: %s", ErrNotFound, entity.ID)
	}

	return nil
}

func (r *ResourcePoolMongoDBWriteRepository) Delete(ctx context.Context, id model.AggregateID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id.String()})
	if err != nil {
		return fmt.Errorf("delete error: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}

	return nil
}

func (r *ResourcePoolMongoDBWriteRepository) BatchSave(ctx context.Context, entities []*model.ResourcePoolDef) error {
	if len(entities) == 0 {
		return nil
	}

	docs := make([]interface{}, len(entities))
	for i, entity := range entities {
		if entity.ID == model.AggregateID(uuid.Nil) {
			entity.ID = model.NewAggregateID()
		}
		docs[i] = r.modelToDocument(entity, ctx)
	}

	_, err := r.collection.InsertMany(ctx, docs)
	if err != nil {
		return fmt.Errorf("batch insert error: %w", err)
	}

	return nil
}

func (r *ResourcePoolMongoDBWriteRepository) BatchUpdate(ctx context.Context, entities []*model.ResourcePoolDef) error {
	if len(entities) == 0 {
		return nil
	}

	var models []mongo.WriteModel
	for _, entity := range entities {
		if entity.ID == model.AggregateID(uuid.Nil) {
			return fmt.Errorf("update requires valid ID")
		}
		// Convertir el modelo a documento
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
