package task_repository

import (
	"context"
	"fmt"
	"time"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// TaskMongoDBWriteRepository implementa operaciones de escritura en MongoDB
var _ ports.WriteOnlyRepository[*model.Task, model.AggregateID] = (*TaskMongoDBWriteRepository)(nil)

type TaskMongoDBWriteRepository struct {
	collection *mongo.Collection
	client     *mongo.Client
}

func NewTaskMongoDBWriteRepository(db *mongo.Database, client *mongo.Client) ports.WriteOnlyRepository[*model.Task, model.AggregateID] {
	return &TaskMongoDBWriteRepository{
		collection: db.Collection("tasks"),
		client:     client,
	}
}

// modelToDocument convierte un modelo de dominio a un documento MongoDB.
// Se asegura que se utilicen los campos de creación y actualización.
func (r *TaskMongoDBWriteRepository) modelToDocument(entity *model.Task, ctx context.Context) TaskDocument {
	owner := r.getOwnerFromContext(ctx)
	tenantID := r.getTenantIDFromContext(ctx)
	now := time.Now().UTC()

	if entity.Metadata.CreatedAt.IsZero() {
		entity.Metadata.CreatedAt = now
	}
	entity.Metadata.UpdatedAt = now

	// Se construyen los parámetros del documento
	params := make([]ParamDefinitionDB, len(entity.Spec.Params))
	for i, param := range entity.Spec.Params {
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

		validations := ParamValidationsDB{
			MinLength:       param.Validations.MinLength,
			MaxLength:       param.Validations.MaxLength,
			Pattern:         param.Validations.Pattern,
			Min:             param.Validations.Min,
			Max:             param.Validations.Max,
			Enum:            param.Validations.Enum,
			Format:          param.Validations.Format,
			CustomValidator: param.Validations.CustomValidator,
		}

		params[i] = ParamDefinitionDB{
			Key:         param.Key,
			Type:        string(param.Type),
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
			WorkerID:    string(entity.Spec.WorkerDefinitionID),
			Command:     entity.Spec.Command,
			Params:      params,
			ParamValues: entity.Spec.ParamValues,
		},
		Owner:     owner,
		TenantID:  tenantID,
		CreatedAt: entity.Metadata.CreatedAt,
		UpdatedAt: now,
	}
}

// Save guarda una nueva Task en la base de datos y devuelve la entidad con el ID asignado.
// Se asigna un ID si la entidad no lo posee.
func (r *TaskMongoDBWriteRepository) Save(ctx context.Context, entity *model.Task) (*model.Task, error) {
	// Verificar que la tarea no exista usando el campo _id
	exists, err := r.exists(ctx, entity.ID)
	if err != nil {
		return nil, fmt.Errorf("error al verificar existencia: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("ya existe una tarea con el ID %s", entity.ID.String())
	}

	doc := r.modelToDocument(entity, ctx)

	_, err = r.collection.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, fmt.Errorf("ya existe una tarea con el ID %s", entity.ID.String())
		}
		return nil, fmt.Errorf("error al guardar tarea: %w", err)
	}

	return entity, nil
}

// Update actualiza una Task existente en la base de datos.
func (r *TaskMongoDBWriteRepository) Update(ctx context.Context, entity *model.Task) error {
	if entity.ID == model.AggregateID("") {
		return fmt.Errorf("no se puede actualizar una entidad sin ID")
	}

	doc := r.modelToDocument(entity, ctx)

	result, err := r.collection.ReplaceOne(ctx, bson.M{"_id": entity.ID.String()}, doc)
	if err != nil {
		return fmt.Errorf("error al actualizar tarea: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("tarea con ID %s no encontrada", entity.ID.String())
	}

	return nil
}

// Delete elimina una Task por su ID.
func (r *TaskMongoDBWriteRepository) Delete(ctx context.Context, id model.AggregateID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id.String()})
	if err != nil {
		return fmt.Errorf("error al eliminar tarea: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("tarea con ID %s no encontrada", id.String())
	}

	return nil
}

// BatchSave guarda múltiples Tasks en la base de datos y devuelve las entidades con sus IDs asignados.
func (r *TaskMongoDBWriteRepository) BatchSave(ctx context.Context, entities []*model.Task) ([]*model.Task, error) {
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

// BatchUpdate actualiza múltiples Tasks en la base de datos.
func (r *TaskMongoDBWriteRepository) BatchUpdate(ctx context.Context, entities []*model.Task) error {
	if len(entities) == 0 {
		return nil
	}

	var models []mongo.WriteModel
	for _, entity := range entities {
		if entity.ID == model.AggregateID("") {
			return fmt.Errorf("update requiere un ID válido")
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

// BatchDelete elimina múltiples Tasks por sus IDs.
func (r *TaskMongoDBWriteRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
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
		return fmt.Errorf("algunas tareas no fueron encontradas")
	}

	return nil
}

// exists verifica si una Task existe por su ID.
func (r *TaskMongoDBWriteRepository) exists(ctx context.Context, id model.AggregateID) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"_id": id.String()})
	if err != nil {
		return false, fmt.Errorf("error al verificar existencia: %w", err)
	}
	return count > 0, nil
}

func (r *TaskMongoDBWriteRepository) getOwnerFromContext(ctx context.Context) string {
	if owner, ok := ctx.Value("owner").(string); ok && owner != "" {
		return owner
	}
	return "system"
}

func (r *TaskMongoDBWriteRepository) getTenantIDFromContext(ctx context.Context) string {
	if tenantID, ok := ctx.Value("tenantID").(string); ok && tenantID != "" {
		return tenantID
	}
	return "default"
}
