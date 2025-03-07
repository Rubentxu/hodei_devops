package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"github.com/google/uuid"
)

type TaskExecutionWriteRepository struct {
	db *sql.DB
}

// NewTaskExecutionWriteRepository crea una instancia del repositorio de escritura para ejecuciones de tareas
func NewTaskExecutionWriteRepository(db *sql.DB) *TaskExecutionWriteRepository {
	return &TaskExecutionWriteRepository{
		db: db,
	}
}

// Save guarda una nueva ejecución de tarea en la base de datos
func (r *TaskExecutionWriteRepository) Save(ctx context.Context, execution model.TaskExecution) error {
	query := `
		INSERT INTO task_executions (
			id, 
			metadata, 
			task_id, 
			worker_id,
			status, 
			input_args
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	// Serializar campos JSON
	metadataBytes, err := json.Marshal(execution.Metadata)
	if err != nil {
		return fmt.Errorf("error al serializar metadata: %w", err)
	}

	statusBytes, err := json.Marshal(execution.Status)
	if err != nil {
		return fmt.Errorf("error al serializar status: %w", err)
	}

	inputArgsBytes, err := json.Marshal(execution.InputArgs)
	if err != nil {
		return fmt.Errorf("error al serializar input_args: %w", err)
	}

	// Ejecutar la consulta
	_, err = r.db.ExecContext(
		ctx,
		query,
		uuid.UUID(execution.ID),
		metadataBytes,
		uuid.UUID(execution.Task.ID),
		uuid.UUID(execution.WorkerDef.ID),
		statusBytes,
		inputArgsBytes,
	)

	if err != nil {
		return fmt.Errorf("error al guardar ejecución de tarea: %w", err)
	}

	return nil
}

// Update actualiza una ejecución de tarea existente
func (r *TaskExecutionWriteRepository) Update(ctx context.Context, execution model.TaskExecution) error {
	query := `
		UPDATE task_executions
		SET 
			metadata = $2, 
			task_id = $3, 
			worker_id = $4,
			status = $5, 
			input_args = $6
		WHERE id = $1
	`

	// Serializar campos JSON
	metadataBytes, err := json.Marshal(execution.Metadata)
	if err != nil {
		return fmt.Errorf("error al serializar metadata: %w", err)
	}

	statusBytes, err := json.Marshal(execution.Status)
	if err != nil {
		return fmt.Errorf("error al serializar status: %w", err)
	}

	inputArgsBytes, err := json.Marshal(execution.InputArgs)
	if err != nil {
		return fmt.Errorf("error al serializar input_args: %w", err)
	}

	// Ejecutar la consulta
	result, err := r.db.ExecContext(
		ctx,
		query,
		uuid.UUID(execution.ID),
		metadataBytes,
		uuid.UUID(execution.Task.ID),
		uuid.UUID(execution.WorkerDef.ID),
		statusBytes,
		inputArgsBytes,
	)

	if err != nil {
		return fmt.Errorf("error al actualizar ejecución de tarea: %w", err)
	}

	// Verificar que se actualizó al menos una fila
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error al obtener filas afectadas: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("ejecución de tarea con ID %s no encontrada", execution.ID)
	}

	return nil
}

// Delete elimina una ejecución de tarea por su ID
func (r *TaskExecutionWriteRepository) Delete(ctx context.Context, id model.AggregateID) error {
	query := `
		DELETE FROM task_executions
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, uuid.UUID(id))
	if err != nil {
		return fmt.Errorf("error al eliminar ejecución de tarea: %w", err)
	}

	// Verificar que se eliminó al menos una fila
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error al obtener filas afectadas: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("ejecución de tarea con ID %s no encontrada", id)
	}

	return nil
}

// BatchSave guarda múltiples ejecuciones de tareas en una transacción
func (r *TaskExecutionWriteRepository) BatchSave(ctx context.Context, executions []model.TaskExecution) error {
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
		INSERT INTO task_executions (
			id, 
			metadata, 
			task_id, 
			worker_id,
			status, 
			input_args
		) VALUES ($1, $2, $3, $4, $5, $6)
	`)
	if err != nil {
		return fmt.Errorf("error al preparar sentencia: %w", err)
	}
	defer stmt.Close()

	for _, execution := range executions {
		// Serializar campos JSON
		metadataBytes, err := json.Marshal(execution.Metadata)
		if err != nil {
			return fmt.Errorf("error al serializar metadata: %w", err)
		}

		statusBytes, err := json.Marshal(execution.Status)
		if err != nil {
			return fmt.Errorf("error al serializar status: %w", err)
		}

		inputArgsBytes, err := json.Marshal(execution.InputArgs)
		if err != nil {
			return fmt.Errorf("error al serializar input_args: %w", err)
		}

		_, err = stmt.ExecContext(
			ctx,
			uuid.UUID(execution.ID),
			metadataBytes,
			uuid.UUID(execution.Task.ID),
			uuid.UUID(execution.WorkerDef.ID),
			statusBytes,
			inputArgsBytes,
		)

		if err != nil {
			return fmt.Errorf("error al guardar ejecución de tarea %s: %w", execution.ID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error al confirmar transacción: %w", err)
	}

	return nil
}

// BatchDelete elimina múltiples ejecuciones de tareas por sus IDs
func (r *TaskExecutionWriteRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
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
		DELETE FROM task_executions
		WHERE id = $1
	`)
	if err != nil {
		return fmt.Errorf("error al preparar sentencia: %w", err)
	}
	defer stmt.Close()

	for _, id := range ids {
		_, err = stmt.ExecContext(ctx, uuid.UUID(id))
		if err != nil {
			return fmt.Errorf("error al eliminar ejecución de tarea %s: %w", id, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error al confirmar transacción: %w", err)
	}

	return nil
}

// WithTransaction ejecuta una función dentro de una transacción
func (r *TaskExecutionWriteRepository) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
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

// UpdateExecutionStatus actualiza solo el estado de una ejecución
func (r *TaskExecutionWriteRepository) UpdateExecutionStatus(ctx context.Context, id model.AggregateID, status model.ExecutionStatus) error {
	query := `
		UPDATE task_executions
		SET status = $2
		WHERE id = $1
	`

	statusBytes, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("error al serializar status: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query, uuid.UUID(id), statusBytes)
	if err != nil {
		return fmt.Errorf("error al actualizar estado de ejecución: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error al obtener filas afectadas: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("ejecución de tarea con ID %s no encontrada", id)
	}

	return nil
}

// Exists verifica si existe una ejecución por su ID
func (r *TaskExecutionWriteRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM task_executions WHERE id = $1)"

	err := r.db.QueryRowContext(ctx, query, uuid.UUID(id)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error al verificar existencia de ejecución: %w", err)
	}

	return exists, nil
}
