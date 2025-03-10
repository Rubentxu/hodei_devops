package repository

import (
	"context"

	"fmt"
	"time"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// TaskMongoDBWriteRepository implementa la interfaz WriteOnlyRepository para Task
var _ ports.WriteOnlyRepository[*model.Task, model.AggregateID] = (*TaskMongoDBWriteRepository)(nil)

// TaskMongoDBWriteRepository implementa operaciones de escritura en MongoDB
type TaskMongoDBWriteRepository struct {
	collection *mongo.Collection
	client     *mongo.Client
}

// NewTaskMongoDBWriteRepository crea una instancia del repositorio de escritura
func NewTaskMongoDBWriteRepository(db *mongo.Database, client *mongo.Client) ports.WriteOnlyRepository[*model.Task, model.AggregateID] {
	return &TaskMongoDBWriteRepository{
		collection: db.Collection("tasks"),
		client:     client,
	}
}

// modelToDocument convierte un modelo de dominio a un documento MongoDB
func (r *TaskMongoDBWriteRepository) modelToDocument(entity *model.Task, ctx context.Context) TaskDocument {
	owner := r.getOwnerFromContext(ctx)
	tenantID := r.getTenantIDFromContext(ctx)
	now := time.Now().UTC()

	// Garantizar que las fechas de creación/actualización existan
	if entity.Metadata.CreatedAt.IsZero() {
		entity.Metadata.CreatedAt = now
	}
	entity.Metadata.UpdatedAt = now

	// Convertir parámetros del modelo a documento
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
			WorkerID:    entity.Spec.WorkerDefinitionID.String(),
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

// Save guarda una nueva Task en la base de datos
func (r *TaskMongoDBWriteRepository) Save(ctx context.Context, entity *model.Task) error {
	// Si no tiene ID, generar uno nuevo
	if entity.ID == model.AggregateID(uuid.Nil) {
		entity.ID = model.NewAggregateID()
	}

	// Validar que la tarea no existe ya
	exists, err := r.exists(ctx, entity.ID)
	if err != nil {
		return fmt.Errorf("error al verificar existencia: %w", err)
	}
	if exists {
		return fmt.Errorf("ya existe una tarea con el ID %s", entity.ID.String())
	}

	// Convertir modelo a documento
	doc := r.modelToDocument(entity, ctx)

	// Insertar en la base de datos
	_, err = r.collection.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("ya existe una tarea con el ID %s", entity.ID.String())
		}
		return fmt.Errorf("error al guardar tarea: %w", err)
	}

	return nil
}

// Update actualiza una Task existente en la base de datos
func (r *TaskMongoDBWriteRepository) Update(ctx context.Context, entity *model.Task) error {
	// Validar que existe un ID
	if entity.ID == model.AggregateID(uuid.Nil) {
		return fmt.Errorf("no se puede actualizar una entidad sin ID")
	}

	// Convertir modelo a documento
	doc := r.modelToDocument(entity, ctx)

	// Actualizar en la base de datos
	result, err := r.collection.ReplaceOne(ctx, bson.M{"id": entity.ID.String()}, doc)
	if err != nil {
		return fmt.Errorf("error al actualizar tarea: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("tarea con ID %s no encontrada", entity.ID.String())
	}

	return nil
}

// Delete elimina una Task por su ID
func (r *TaskMongoDBWriteRepository) Delete(ctx context.Context, id model.AggregateID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"id": id.String()})
	if err != nil {
		return fmt.Errorf("error al eliminar tarea: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("tarea con ID %s no encontrada", id.String())
	}

	return nil
}

// BatchSave guarda múltiples Tasks en la base de datos
func (r *TaskMongoDBWriteRepository) BatchSave(ctx context.Context, entities []*model.Task) error {
	if len(entities) == 0 {
		return nil // No hay entidades para guardar
	}

	return r.WithTransaction(ctx, func(txCtx context.Context) error {
		for _, entity := range entities {
			if err := r.Save(txCtx, entity); err != nil {
				return err
			}
		}
		return nil
	})
}

// BatchUpdate actualiza múltiples Tasks en la base de datos
func (r *TaskMongoDBWriteRepository) BatchUpdate(ctx context.Context, entities []*model.Task) error {
	if len(entities) == 0 {
		return nil // No hay entidades para actualizar
	}

	return r.WithTransaction(ctx, func(txCtx context.Context) error {
		for _, entity := range entities {
			if err := r.Update(txCtx, entity); err != nil {
				return err
			}
		}
		return nil
	})
}

// BatchDelete elimina múltiples Tasks por sus IDs
func (r *TaskMongoDBWriteRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	if len(ids) == 0 {
		return nil // No hay IDs para eliminar
	}

	return r.WithTransaction(ctx, func(txCtx context.Context) error {
		for _, id := range ids {
			// Verificamos si existe antes de eliminar para no devolver error si no existe
			exists, err := r.exists(txCtx, id)
			if err != nil {
				return fmt.Errorf("error al verificar existencia del ID %s: %w", id.String(), err)
			}

			if exists {
				if err := r.Delete(txCtx, id); err != nil {
					return err
				}
			} else {
				// Log warning pero continuamos con la eliminación de otros IDs
				fmt.Printf("Warning: tarea con ID %s no encontrada, continuando con otros IDs\n", id.String())
			}
		}
		return nil
	})
}

// WithTransaction ejecuta una función dentro de una transacción
func (r *TaskMongoDBWriteRepository) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	// Configurar opciones de transacción
	wc := writeconcern.New(writeconcern.WMajority())
	txnOptions := options.Transaction().SetWriteConcern(wc)

	// Iniciar la sesión
	session, err := r.client.StartSession()
	if err != nil {
		return fmt.Errorf("error al iniciar sesión de transacción: %w", err)
	}
	defer session.EndSession(ctx)

	// Ejecutar la transacción
	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		return nil, fn(sessCtx)
	}, txnOptions)

	if err != nil {
		return fmt.Errorf("error en la transacción: %w", err)
	}

	return nil
}

// exists verifica si una Task existe por su ID
func (r *TaskMongoDBWriteRepository) exists(ctx context.Context, id model.AggregateID) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"id": id.String()})
	if err != nil {
		return false, fmt.Errorf("error al verificar existencia: %w", err)
	}
	return count > 0, nil
}

// getOwnerFromContext obtiene el propietario del recurso del contexto, o usa valor por defecto
func (r *TaskMongoDBWriteRepository) getOwnerFromContext(ctx context.Context) string {
	if owner, ok := ctx.Value("owner").(string); ok && owner != "" {
		return owner
	}
	return "system" // Valor por defecto
}

// getTenantIDFromContext obtiene el ID de inquilino del contexto, o usa valor por defecto
func (r *TaskMongoDBWriteRepository) getTenantIDFromContext(ctx context.Context) string {
	if tenantID, ok := ctx.Value("tenantID").(string); ok && tenantID != "" {
		return tenantID
	}
	return "default" // Valor por defecto
}
