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

// ResourcePoolMongoDBWriteRepository implementa la interfaz WriteOnlyRepository para ResourcePoolDef
var _ ports.WriteOnlyRepository[*model.ResourcePoolDef, model.AggregateID] = (*ResourcePoolMongoDBWriteRepository)(nil)

// ResourcePoolMongoDBWriteRepository implementa operaciones de escritura en MongoDB
type ResourcePoolMongoDBWriteRepository struct {
	collection *mongo.Collection
	client     *mongo.Client
}

// NewResourcePoolMongoDBWriteRepository crea una instancia del repositorio de escritura
func NewResourcePoolMongoDBWriteRepository(db *mongo.Database, client *mongo.Client) ports.WriteOnlyRepository[*model.ResourcePoolDef, model.AggregateID] {
	return &ResourcePoolMongoDBWriteRepository{
		collection: db.Collection("resource_pools"),
		client:     client,
	}
}

// modelToDocument convierte un modelo de dominio a un documento MongoDB
func (r *ResourcePoolMongoDBWriteRepository) modelToDocument(entity *model.ResourcePoolDef, ctx context.Context) ResourcePoolDocument {
	owner := r.getOwnerFromContext(ctx)
	tenantID := r.getTenantIDFromContext(ctx)
	now := time.Now().UTC()

	// Garantizar que las fechas de creación/actualización existan
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
		Spec: ResourcePoolSpecDB{
			PoolID: entity.Spec.PoolID,
			Type:   entity.Spec.Type,
			Config: entity.Spec.ExtendedSpec,
		},
		Status: ResourcePoolStatus{
			State: entity.Status.State,
		},
		Owner:     owner,
		TenantID:  tenantID,
		CreatedAt: entity.Metadata.CreatedAt,
		UpdatedAt: now,
	}
}

// Save guarda un nuevo ResourcePoolDef en la base de datos
func (r *ResourcePoolMongoDBWriteRepository) Save(ctx context.Context, entity *model.ResourcePoolDef) error {
	// Si no tiene ID, generar uno nuevo
	if entity.ID == model.AggregateID(uuid.Nil) {
		entity.ID = model.NewAggregateID()
	}

	// Validar que el pool no existe ya
	exists, err := r.exists(ctx, entity.ID)
	if err != nil {
		return fmt.Errorf("error al verificar existencia: %w", err)
	}
	if exists {
		return fmt.Errorf("ya existe un resource pool con el ID %s", entity.ID.String())
	}

	// Convertir modelo a documento
	doc := r.modelToDocument(entity, ctx)

	// Insertar en la base de datos
	_, err = r.collection.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("ya existe un resource pool con el ID %s", entity.ID.String())
		}
		return fmt.Errorf("error al guardar resource pool: %w", err)
	}

	return nil
}

// Update actualiza un ResourcePoolDef existente en la base de datos
func (r *ResourcePoolMongoDBWriteRepository) Update(ctx context.Context, entity *model.ResourcePoolDef) error {
	// Validar que existe un ID
	if entity.ID == model.AggregateID(uuid.Nil) {
		return fmt.Errorf("no se puede actualizar una entidad sin ID")
	}

	// Convertir modelo a documento
	doc := r.modelToDocument(entity, ctx)

	// Actualizar en la base de datos
	result, err := r.collection.ReplaceOne(ctx, bson.M{"id": entity.ID.String()}, doc)
	if err != nil {
		return fmt.Errorf("error al actualizar resource pool: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("resource pool con ID %s no encontrado", entity.ID.String())
	}

	return nil
}

// Delete elimina un ResourcePoolDef por su ID
func (r *ResourcePoolMongoDBWriteRepository) Delete(ctx context.Context, id model.AggregateID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"id": id.String()})
	if err != nil {
		return fmt.Errorf("error al eliminar resource pool: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("resource pool con ID %s no encontrado", id.String())
	}

	return nil
}

// BatchSave guarda múltiples ResourcePoolDef en la base de datos
func (r *ResourcePoolMongoDBWriteRepository) BatchSave(ctx context.Context, entities []*model.ResourcePoolDef) error {
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

// BatchUpdate actualiza múltiples ResourcePoolDef en la base de datos
func (r *ResourcePoolMongoDBWriteRepository) BatchUpdate(ctx context.Context, entities []*model.ResourcePoolDef) error {
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

// BatchDelete elimina múltiples ResourcePoolDef por sus IDs
func (r *ResourcePoolMongoDBWriteRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
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
				fmt.Printf("Warning: resource pool con ID %s no encontrado, continuando con otros IDs\n", id.String())
			}
		}
		return nil
	})
}

// WithTransaction ejecuta una función dentro de una transacción
func (r *ResourcePoolMongoDBWriteRepository) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
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

// exists verifica si un ResourcePoolDef existe por su ID
func (r *ResourcePoolMongoDBWriteRepository) exists(ctx context.Context, id model.AggregateID) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"id": id.String()})
	if err != nil {
		return false, fmt.Errorf("error al verificar existencia: %w", err)
	}
	return count > 0, nil
}

// getOwnerFromContext obtiene el propietario del recurso del contexto, o usa valor por defecto
func (r *ResourcePoolMongoDBWriteRepository) getOwnerFromContext(ctx context.Context) string {
	if owner, ok := ctx.Value("owner").(string); ok && owner != "" {
		return owner
	}
	return "system" // Valor por defecto
}

// getTenantIDFromContext obtiene el ID de inquilino del contexto, o usa valor por defecto
func (r *ResourcePoolMongoDBWriteRepository) getTenantIDFromContext(ctx context.Context) string {
	if tenantID, ok := ctx.Value("tenantID").(string); ok && tenantID != "" {
		return tenantID
	}
	return "default" // Valor por defecto
}
