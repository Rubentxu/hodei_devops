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

type TaskWriteRepository struct {
	db *sql.DB
}

// NewTaskWriteRepository crea una nueva instancia del repositorio de escritura para tareas
func NewTaskWriteRepository(db *sql.DB) *TaskWriteRepository {
	return &TaskWriteRepository{
		db: db,
	}
}

// Save almacena una nueva tarea en la base de datos
func (r *TaskWriteRepository) Save(ctx context.Context, entity model.Task) error {
	// Preparar los datos JSON
	metadata, err := json.Marshal(entity.Metadata)
	if err != nil {
		return fmt.Errorf("error al serializar metadata: %w", err)
	}

	// Preparar Spec
	spec := map[string]interface{}{
		"worker_id":    entity.Spec.WorkerDefinitionID,
		"command":      entity.Spec.Command,
		"params":       entity.Spec.Params,
		"param_values": entity.Spec.ParamValues,
	}

	specJSON, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("error al serializar spec: %w", err)
	}

	// Insertar en la base de datos
	query := `
		INSERT INTO tasks (id, metadata, spec)
		VALUES ($1, $2, $3)
	`

	_, err = r.db.ExecContext(ctx, query, uuid.UUID(entity.ID), metadata, specJSON)
	if err != nil {
		return fmt.Errorf("error al guardar la tarea: %w", err)
	}

	return nil
}

// Update actualiza una tarea existente en la base de datos
func (r *TaskWriteRepository) Update(ctx context.Context, entity model.Task) error {
	// Verificar si la tarea existe
	exists, err := r.Exists(ctx, entity.ID)
	if err != nil {
		return fmt.Errorf("error al verificar existencia de la tarea: %w", err)
	}
	if !exists {
		return fmt.Errorf("la tarea con ID %s no existe", entity.ID)
	}

	// Preparar los datos JSON
	metadata, err := json.Marshal(entity.Metadata)
	if err != nil {
		return fmt.Errorf("error al serializar metadata: %w", err)
	}

	// Preparar Spec
	spec := map[string]interface{}{
		"worker_id":    entity.Spec.WorkerDefinitionID,
		"command":      entity.Spec.Command,
		"params":       entity.Spec.Params,
		"param_values": entity.Spec.ParamValues,
	}

	specJSON, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("error al serializar spec: %w", err)
	}

	// Actualizar en la base de datos
	query := `
		UPDATE tasks 
		SET metadata = $2, spec = $3
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, uuid.UUID(entity.ID), metadata, specJSON)
	if err != nil {
		return fmt.Errorf("error al actualizar la tarea: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error al obtener filas afectadas: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("no se actualizó ninguna tarea")
	}

	return nil
}

// Delete elimina una tarea por su ID
func (r *TaskWriteRepository) Delete(ctx context.Context, id model.AggregateID) error {
	query := `
		DELETE FROM tasks 
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, uuid.UUID(id))
	if err != nil {
		return fmt.Errorf("error al eliminar la tarea: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error al obtener filas afectadas: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("no se eliminó ninguna tarea")
	}

	return nil
}

// BatchSave almacena múltiples tareas en una sola operación
func (r *TaskWriteRepository) BatchSave(ctx context.Context, entities []model.Task) error {
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
		INSERT INTO tasks (id, metadata, spec)
		VALUES ($1, $2, $3)
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

		// Preparar Spec
		spec := map[string]interface{}{
			"worker_id":    entity.Spec.WorkerDefinitionID,
			"command":      entity.Spec.Command,
			"params":       entity.Spec.Params,
			"param_values": entity.Spec.ParamValues,
		}

		specJSON, err := json.Marshal(spec)
		if err != nil {
			return fmt.Errorf("error al serializar spec: %w", err)
		}

		_, err = stmt.ExecContext(ctx, uuid.UUID(entity.ID), metadata, specJSON)
		if err != nil {
			return fmt.Errorf("error al insertar tarea %s: %w", entity.ID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error al confirmar transacción: %w", err)
	}

	return nil
}

// BatchDelete elimina múltiples tareas por sus IDs
func (r *TaskWriteRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
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
		DELETE FROM tasks 
		WHERE id = $1
	`)
	if err != nil {
		return fmt.Errorf("error al preparar sentencia: %w", err)
	}
	defer stmt.Close()

	for _, id := range ids {
		_, err = stmt.ExecContext(ctx, uuid.UUID(id))
		if err != nil {
			return fmt.Errorf("error al eliminar tarea %s: %w", id, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error al confirmar transacción: %w", err)
	}

	return nil
}

// WithTransaction ejecuta una función dentro de una transacción
func (r *TaskWriteRepository) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
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

// Exists verifica si una tarea existe por su ID
func (r *TaskWriteRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS(SELECT 1 FROM tasks WHERE id = $1)
	`

	err := r.db.QueryRowContext(ctx, query, uuid.UUID(id)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error al verificar existencia de tarea: %w", err)
	}

	return exists, nil
}
