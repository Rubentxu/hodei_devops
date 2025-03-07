package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/google/uuid"
)

// ResourcePoolWriteRepository implementa la interfaz WriteOnlyRepository para ResourcePoolDef
var _ ports.WriteOnlyRepository[*model.ResourcePoolDef, model.AggregateID] = (*ResourcePoolWriteRepository)(nil)

type ResourcePoolWriteRepository struct {
	db *sql.DB
}

// NewResourcePoolWriteRepository crea una instancia del repositorio de escritura
func NewResourcePoolWriteRepository(db *sql.DB) ports.WriteOnlyRepository[*model.ResourcePoolDef, model.AggregateID] {
	return &ResourcePoolWriteRepository{
		db: db,
	}
}

// Save guarda un nuevo ResourcePoolDef en la base de datos
func (r *ResourcePoolWriteRepository) Save(ctx context.Context, entity *model.ResourcePoolDef) error {
	// Si no tiene ID, generar uno nuevo
	if entity.ID == model.AggregateID(uuid.Nil) {
		entity.ID = model.AggregateID(uuid.New())
	}

	// Convertir a JSON para almacenar en PostgreSQL
	metadata, err := json.Marshal(entity.Metadata)
	if err != nil {
		return fmt.Errorf("error al serializar metadata: %w", err)
	}

	spec, err := json.Marshal(entity.Spec)
	if err != nil {
		return fmt.Errorf("error al serializar spec: %w", err)
	}

	status, err := json.Marshal(entity.Status)
	if err != nil {
		return fmt.Errorf("error al serializar status: %w", err)
	}

	query := `
		INSERT INTO resource_pools (id, metadata, spec, status)
		VALUES ($1, $2, $3, $4)
	`

	_, err = r.db.ExecContext(ctx, query, entity.ID.String(), metadata, spec, status)
	if err != nil {
		if strings.Contains(err.Error(), "check_metadata_format") {
			return fmt.Errorf("formato de metadata inválido: %w", err)
		} else if strings.Contains(err.Error(), "check_spec_format") {
			return fmt.Errorf("formato de spec inválido: %w", err)
		} else if strings.Contains(err.Error(), "check_status_format") {
			return fmt.Errorf("formato de status inválido: %w", err)
		}
		return fmt.Errorf("error al guardar resource pool: %w", err)
	}

	return nil
}

// Update actualiza un ResourcePoolDef existente en la base de datos
func (r *ResourcePoolWriteRepository) Update(ctx context.Context, entity *model.ResourcePoolDef) error {
	// Validar que existe un ID
	if entity.ID == model.AggregateID(uuid.Nil) {
		return fmt.Errorf("no se puede actualizar una entidad sin ID")
	}

	// Actualizar timestamp
	now := time.Now().UTC()
	entity.Metadata.UpdatedAt = now

	// Convertir a JSON para almacenar en PostgreSQL
	metadata, err := json.Marshal(entity.Metadata)
	if err != nil {
		return fmt.Errorf("error al serializar metadata: %w", err)
	}

	spec, err := json.Marshal(entity.Spec)
	if err != nil {
		return fmt.Errorf("error al serializar spec: %w", err)
	}

	status, err := json.Marshal(entity.Status)
	if err != nil {
		return fmt.Errorf("error al serializar status: %w", err)
	}

	query := `
		UPDATE resource_pools 
		SET metadata = $2, spec = $3, status = $4
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, entity.ID.String(), metadata, spec, status)
	if err != nil {
		if strings.Contains(err.Error(), "check_metadata_format") {
			return fmt.Errorf("formato de metadata inválido: %w", err)
		} else if strings.Contains(err.Error(), "check_spec_format") {
			return fmt.Errorf("formato de spec inválido: %w", err)
		} else if strings.Contains(err.Error(), "check_status_format") {
			return fmt.Errorf("formato de status inválido: %w", err)
		}
		return fmt.Errorf("error al actualizar resource pool: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error al verificar filas afectadas: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("resource pool con ID %s no encontrado", entity.ID.String())
	}

	return nil
}

// Delete elimina un ResourcePoolDef por su ID
func (r *ResourcePoolWriteRepository) Delete(ctx context.Context, id model.AggregateID) error {
	query := "DELETE FROM resource_pools WHERE id = $1"

	result, err := r.db.ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("error al eliminar resource pool: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error al verificar filas afectadas: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("resource pool con ID %s no encontrado", id.String())
	}

	return nil
}

// BatchSave guarda múltiples ResourcePoolDef en la base de datos
func (r *ResourcePoolWriteRepository) BatchSave(ctx context.Context, entities []*model.ResourcePoolDef) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error al iniciar transacción: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	for _, entity := range entities {
		// Creamos un contexto temporal para la transacción
		if err := r.saveInTx(ctx, tx, entity); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// saveInTx es un método auxiliar para guardar una entidad en una transacción existente
func (r *ResourcePoolWriteRepository) saveInTx(ctx context.Context, tx *sql.Tx, entity *model.ResourcePoolDef) error {
	// Si no tiene ID, generar uno nuevo
	if entity.ID == model.AggregateID(uuid.Nil) {
		entity.ID = model.AggregateID(uuid.New())
	}

	// Convertir a JSON para almacenar en PostgreSQL
	metadata, err := json.Marshal(entity.Metadata)
	if err != nil {
		return fmt.Errorf("error al serializar metadata: %w", err)
	}

	spec, err := json.Marshal(entity.Spec)
	if err != nil {
		return fmt.Errorf("error al serializar spec: %w", err)
	}

	status, err := json.Marshal(entity.Status)
	if err != nil {
		return fmt.Errorf("error al serializar status: %w", err)
	}

	query := `
		INSERT INTO resource_pools (id, metadata, spec, status)
		VALUES ($1, $2, $3, $4)
	`

	_, err = tx.ExecContext(ctx, query, entity.ID.String(), metadata, spec, status)
	// Ejemplo de manejo de errores en Save
	if err != nil {
		if strings.Contains(err.Error(), "check_metadata_format") {
			return fmt.Errorf("formato de metadata inválido: %w", err)
		} else if strings.Contains(err.Error(), "check_spec_format") {
			return fmt.Errorf("formato de spec inválido: %w", err)
		} else if strings.Contains(err.Error(), "check_status_format") {
			return fmt.Errorf("formato de status inválido: %w", err)
		}
		return fmt.Errorf("error al guardar resource pool en transacción: %w", err)
	}
	return nil
}

// BatchDelete elimina múltiples ResourcePoolDef por sus IDs
func (r *ResourcePoolWriteRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error al iniciar transacción: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	query := "DELETE FROM resource_pools WHERE id = $1"
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error al preparar statement: %w", err)
	}
	defer stmt.Close()

	for _, id := range ids {
		_, err := stmt.ExecContext(ctx, id.String())
		if err != nil {
			return fmt.Errorf("error al eliminar resource pool %s: %w", id.String(), err)
		}
	}

	return tx.Commit()
}

// WithTransaction ejecuta una función dentro de una transacción
func (r *ResourcePoolWriteRepository) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error al iniciar transacción: %w", err)
	}

	// Crear un contexto con el valor de la transacción
	txCtx := context.WithValue(ctx, "tx", tx)

	// Ejecutar la función proporcionada
	if err := fn(txCtx); err != nil {
		tx.Rollback()
		return err
	}

	// Confirmar la transacción si todo fue bien
	return tx.Commit()
}
