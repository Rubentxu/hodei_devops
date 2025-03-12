package workerdef_repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrDuplicateID = errors.New("ID de worker duplicado")
	ErrMissingID   = errors.New("ID de worker no especificado")
)

const (
	ctxKeyOwner    = "owner"
	ctxKeyTenantID = "tenantID"
)

// Se implementa la interfaz WriteOnlyRepository, donde Save devuelve la entidad con el ID asignado.
var _ ports.WriteOnlyRepository[*model.WorkerDefinition, model.AggregateID] = (*WorkerMongoDBWriteRepository)(nil)

type WorkerMongoDBWriteRepository struct {
	collection *mongo.Collection
	client     *mongo.Client
}

func NewWorkerMongoDBWriteRepository(db *mongo.Database, client *mongo.Client) ports.WriteOnlyRepository[*model.WorkerDefinition, model.AggregateID] {
	return &WorkerMongoDBWriteRepository{
		collection: db.Collection("workers"),
		client:     client,
	}
}

func healthStatusToString(status model.HealthStatus) string {
	switch status {
	case model.UNKNOWN:
		return "UNKNOWN"
	case model.RUNNING:
		return "RUNNING"
	case model.HEALTHY:
		return "HEALTHY"
	case model.ERROR:
		return "ERROR"
	case model.STOPPED:
		return "STOPPED"
	case model.FINISHED:
		return "FINISHED"
	case model.PENDING:
		return "PENDING"
	case model.DONE:
		return "DONE"
	default:
		return "UNKNOWN"
	}
}

// modelToDocument convierte el modelo de dominio a un documento MongoDB.
func (r *WorkerMongoDBWriteRepository) modelToDocument(entity *model.WorkerDefinition, ctx context.Context) WorkerDocument {
	owner := r.getOwnerFromContext(ctx)
	tenantID := r.getTenantIDFromContext(ctx)
	now := time.Now().UTC()

	if entity.Metadata.CreatedAt.IsZero() {
		entity.Metadata.CreatedAt = now
	}
	entity.Metadata.UpdatedAt = now

	volumes := make([]VolumeMountDB, len(entity.Spec.Volumes))
	for i, v := range entity.Spec.Volumes {
		volumes[i] = VolumeMountDB{
			HostPath:      v.HostPath,
			ContainerPath: v.ContainerPath,
			ReadOnly:      v.ReadOnly,
		}
	}

	ports := make([]PortMappingDB, len(entity.Spec.Ports))
	for i, p := range entity.Spec.Ports {
		ports[i] = PortMappingDB{
			HostPort:      p.HostPort,
			ContainerPort: p.ContainerPort,
			Protocol:      p.Protocol,
		}
	}

	var healthCheck *HealthCheckConfigDB
	if entity.Spec.HealthCheck != nil {
		healthCheck = &HealthCheckConfigDB{
			Type:     entity.Spec.HealthCheck.Type,
			Endpoint: entity.Spec.HealthCheck.Endpoint,
			Interval: int64(entity.Spec.HealthCheck.Interval.Milliseconds()),
			Timeout:  int64(entity.Spec.HealthCheck.Timeout.Milliseconds()),
		}
	}

	return WorkerDocument{
		ID: entity.ID.String(),
		Metadata: WorkerMeta{
			Name:        entity.Metadata.Name,
			Description: entity.Metadata.Description,
			Labels:      entity.Metadata.Labels,
			Annotations: entity.Metadata.Annotations,
			CreatedAt:   entity.Metadata.CreatedAt,
			UpdatedAt:   entity.Metadata.UpdatedAt,
		},
		Spec: WorkerSpecDB{
			Type:       string(entity.Spec.Type),
			Image:      entity.Spec.Image,
			Env:        entity.Spec.Env,
			WorkingDir: entity.Spec.WorkingDir,
			Resources: ResourceRequirementsDB{
				CPU:    entity.Spec.Resources.CPU,
				Memory: entity.Spec.Resources.Memory,
			},
			Volumes:     volumes,
			Ports:       ports,
			Labels:      entity.Spec.Labels,
			HealthCheck: healthCheck,
			TemplateID:  entity.Spec.TemplateID,
		},
		Status: WorkerStatusDB{
			InstanceID: entity.Status.InstanceID,
			Status:     healthStatusToString(entity.Status.Status),
		},
		Owner:     owner,
		TenantID:  tenantID,
		CreatedAt: entity.Metadata.CreatedAt,
		UpdatedAt: now,
	}
}

// Save asigna un ID si es necesario, guarda el worker y devuelve la entidad completa.
func (r *WorkerMongoDBWriteRepository) Save(ctx context.Context, entity *model.WorkerDefinition) (*model.WorkerDefinition, error) {

	doc := r.modelToDocument(entity, ctx)

	_, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateID, entity.ID.String())
		}
		return nil, fmt.Errorf("error al guardar worker: %w", err)
	}

	return entity, nil
}

// Update actualiza los campos del worker.
func (r *WorkerMongoDBWriteRepository) Update(ctx context.Context, entity *model.WorkerDefinition) error {
	if entity.ID == model.AggregateID("") {
		return fmt.Errorf("%w", ErrMissingID)
	}

	doc := r.modelToDocument(entity, ctx)
	updateDoc := bson.M{"$set": doc}
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": entity.ID.String()}, updateDoc)
	if err != nil {
		return fmt.Errorf("error al actualizar worker: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("%w: %s", ErrWorkerNotFound, entity.ID.String())
	}

	return nil
}

// Delete elimina el worker por su ID.
func (r *WorkerMongoDBWriteRepository) Delete(ctx context.Context, id model.AggregateID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id.String()})
	if err != nil {
		return fmt.Errorf("error al eliminar worker: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("%w: %s", ErrWorkerNotFound, id.String())
	}

	return nil
}

// BatchSave asigna IDs en caso de faltar y guarda múltiples workers, devolviendo las entidades con sus IDs asignados.
func (r *WorkerMongoDBWriteRepository) BatchSave(ctx context.Context, entities []*model.WorkerDefinition) ([]*model.WorkerDefinition, error) {
	if len(entities) == 0 {
		return nil, nil
	}

	documents := make([]interface{}, len(entities))
	for i, entity := range entities {
		documents[i] = r.modelToDocument(entity, ctx)
	}

	_, err := r.collection.InsertMany(ctx, documents, options.InsertMany().SetOrdered(false))
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, fmt.Errorf("%w: uno o más IDs ya existen", ErrDuplicateID)
		}
		return nil, fmt.Errorf("error al guardar workers por lote: %w", err)
	}

	return entities, nil
}

// BatchUpdate realiza la actualización masiva de workers.
func (r *WorkerMongoDBWriteRepository) BatchUpdate(ctx context.Context, entities []*model.WorkerDefinition) error {
	if len(entities) == 0 {
		return nil
	}

	var bulkOps []mongo.WriteModel
	for _, entity := range entities {
		if entity.ID == model.AggregateID("") {
			return fmt.Errorf("%w para una entidad en BatchUpdate", ErrMissingID)
		}
		doc := r.modelToDocument(entity, ctx)
		updateModel := mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": entity.ID.String()}).
			SetUpdate(bson.M{"$set": doc})
		bulkOps = append(bulkOps, updateModel)
	}

	opts := options.BulkWrite().SetOrdered(false)
	result, err := r.collection.BulkWrite(ctx, bulkOps, opts)
	if err != nil {
		return fmt.Errorf("error en operación BatchUpdate: %w", err)
	}
	if result.MatchedCount != int64(len(entities)) {
		return fmt.Errorf("actualización fallida: %d de %d workers no encontrados", len(entities)-int(result.MatchedCount), len(entities))
	}

	return nil
}

// BatchDelete elimina múltiples workers por sus IDs.
func (r *WorkerMongoDBWriteRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	if len(ids) == 0 {
		return nil
	}

	stringIDs := make([]string, len(ids))
	for i, id := range ids {
		stringIDs[i] = id.String()
	}

	filter := bson.M{"_id": bson.M{"$in": stringIDs}}
	result, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("error al eliminar workers por lote: %w", err)
	}
	if result.DeletedCount != int64(len(ids)) {
		return fmt.Errorf("solo se eliminaron %d de %d workers", result.DeletedCount, len(ids))
	}
	return nil
}

func (r *WorkerMongoDBWriteRepository) getOwnerFromContext(ctx context.Context) string {
	if owner, ok := ctx.Value(ctxKeyOwner).(string); ok && owner != "" {
		return owner
	}
	return "system"
}

func (r *WorkerMongoDBWriteRepository) getTenantIDFromContext(ctx context.Context) string {
	if tenantID, ok := ctx.Value(ctxKeyTenantID).(string); ok && tenantID != "" {
		return tenantID
	}
	return "default"
}
