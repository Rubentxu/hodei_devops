package repository

import (
	"context"
	"database/sql"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
)

type WorkerWriteRepository struct {
	db *sql.DB
}

// NewWorkerWriteRepository crea una nueva instancia del repositorio de escritura para workers
func NewWorkerWriteRepository(db *sql.DB) *WorkerWriteRepository {
	return &WorkerWriteRepository{
		db: db,
	}
}

// Save almacena un nuevo worker en la base de datos
func (r *WorkerWriteRepository) Save(ctx context.Context, entity model.WorkerDefinition) error {
	// Preparar los datos JSON
	metadata, err := json.Marshal(entity.Metadata)
	if err != nil {
		return fmt.Errorf("error al serializar metadata: %w", err)
	}

	// Preparar WorkerSpec
	spec, err := json.Marshal(entity.Spec)
	if err != nil {
		return fmt.Errorf("error al serializar spec: %w", err)
	}

	// Preparar WorkerStatus
	status, err := json.Marshal(entity.Status)
	if err != nil {
		return fmt.Errorf("error al serializar status: %w", err)
	}

	// Insertar en la base de datos
	query := `
		INSERT INTO workers (id, metadata, spec, status)
		VALUES ($1, $2, $3, $4)
	`

	_, err = r.db.ExecContext(ctx, query, uuid.UUID(entity.ID), metadata, spec, status)
	if err != nil {
		return fmt.Errorf("error al guardar el worker: %w", err)
	}

	return nil
}

// Update actualiza un worker existente en la base de datos
func (r *WorkerWriteRepository) Update(ctx context.Context, entity model.WorkerDefinition) error {
	// Verificar si el worker existe
	exists, err := r.Exists(ctx, entity.ID)
	if err != nil {
		return fmt.Errorf("error al verificar existencia del worker: %w", err)
	}
	if !exists {
		return fmt.Errorf("el worker con ID %s no existe", entity.ID)
	}

	// Preparar los datos JSON
	metadata, err := json.Marshal(entity.Metadata)
	if err != nil {
		return fmt.Errorf("error al serializar metadata: %w", err)
	}

	// Preparar WorkerSpec
	spec, err := json.Marshal(entity.Spec)
	if err != nil {
		return fmt.Errorf("error al serializar spec: %w", err)
	}

	// Preparar WorkerStatus
	status, err := json.Marshal(entity.Status)
	if err != nil {
		return fmt.Errorf("error al serializar status: %w", err)
	}

	// Actualizar en la base de datos
	query := `
		UPDATE workers
		SET metadata = $2, spec = $3, status = $4
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, uuid.UUID(entity.ID), metadata, spec, status)
	if err != nil {
		return fmt.Errorf("error al actualizar el worker: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error al obtener filas afectadas: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("no se actualizó ningún worker")
	}

	return nil
}

// Delete elimina un worker por su ID
func (r *WorkerWriteRepository) Delete(ctx context.Context, id model.AggregateID) error {
	query := `
		DELETE FROM workers
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, uuid.UUID(id))
	if err != nil {
		return fmt.Errorf("error al eliminar el worker: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error al obtener filas afectadas: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("no se eliminó ningún worker")
	}

	return nil
}

// BatchSave almacena múltiples workers en una sola operación
func (r *WorkerWriteRepository) BatchSave(ctx context.Context, entities []model.WorkerDefinition) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error al iniciar transacción: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO workers (id, metadata, spec, status)
		VALUES ($1, $2, $3, $4)
	`)
	if err != nil {
		return fmt.Errorf("error al preparar sentencia: %w", err)
	}
	defer stmt.Close()

	for _, entity := range entities {
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

		_, err = stmt.ExecContext(ctx, uuid.UUID(entity.ID), metadata, spec, status)
		if err != nil {
			return fmt.Errorf("error al insertar worker %s: %w", entity.ID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error al confirmar transacción: %w", err)
	}

	return nil
}

// BatchDelete elimina múltiples workers por sus IDs
func (r *WorkerWriteRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error al iniciar transacción: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		DELETE FROM workers
		WHERE id = $1
	`)
	if err != nil {
		return fmt.Errorf("error al preparar sentencia: %w", err)
	}
	defer stmt.Close()

	for _, id := range ids {
		_, err = stmt.ExecContext(ctx, uuid.UUID(id))
		if err != nil {
			return fmt.Errorf("error al eliminar worker %s: %w", id, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error al confirmar transacción: %w", err)
	}

	return nil
}

// WithTransaction ejecuta una función dentro de una transacción
func (r *WorkerWriteRepository) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error al iniciar transacción: %w", err)
	}

	txCtx := context.WithValue(ctx, "tx", tx)

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	err = fn(txCtx)
	return err
}

// Exists verifica si un worker existe por su ID
func (r *WorkerWriteRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS(SELECT 1 FROM workers WHERE id = $1)
	`

	err := r.db.QueryRowContext(ctx, query, uuid.UUID(id)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error al verificar existencia de worker: %w", err)
	}

	return exists, nil
}
