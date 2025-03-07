package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/google/uuid"
)

type TaskExecutionReadRepository struct {
	db *sql.DB
}

// NewTaskExecutionReadRepository crea una instancia del repositorio de lectura para ejecuciones de tareas
func NewTaskExecutionReadRepository(db *sql.DB) *TaskExecutionReadRepository {
	return &TaskExecutionReadRepository{
		db: db,
	}
}

// FindByID busca una ejecución de tarea por su ID
func (r *TaskExecutionReadRepository) FindByID(ctx context.Context, id model.AggregateID) (model.TaskExecution, error) {
	query := `
		SELECT
			e.id,
			e.metadata,
			e.task_id,
			e.worker_id,
			e.status,
			e.input_args
		FROM
			task_executions e
		WHERE
			e.id = $1
	`

	var execution model.TaskExecution
	var metadataBytes, statusBytes, inputArgsBytes []byte
	var taskID, workerID uuid.UUID

	err := r.db.QueryRowContext(ctx, query, uuid.UUID(id)).Scan(
		&execution.ID,
		&metadataBytes,
		&taskID,
		&workerID,
		&statusBytes,
		&inputArgsBytes,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return model.TaskExecution{}, fmt.Errorf("ejecución de tarea con ID %s no encontrada", id)
		}
		return model.TaskExecution{}, fmt.Errorf("error al buscar ejecución: %w", err)
	}

	// Deserializar metadata
	if err := json.Unmarshal(metadataBytes, &execution.Metadata); err != nil {
		return model.TaskExecution{}, fmt.Errorf("error al deserializar metadata: %w", err)
	}

	// Deserializar status
	if err := json.Unmarshal(statusBytes, &execution.Status); err != nil {
		return model.TaskExecution{}, fmt.Errorf("error al deserializar status: %w", err)
	}

	// Deserializar input_args
	if err := json.Unmarshal(inputArgsBytes, &execution.InputArgs); err != nil {
		return model.TaskExecution{}, fmt.Errorf("error al deserializar input_args: %w", err)
	}

	// Obtener la tarea completa
	task, err := r.getTaskByID(ctx, model.AggregateID(taskID))
	if err != nil {
		return model.TaskExecution{}, err
	}
	execution.Task = task

	// Obtener el worker completo
	worker, err := r.getWorkerByID(ctx, model.AggregateID(workerID))
	if err != nil {
		return model.TaskExecution{}, err
	}
	execution.WorkerDef = worker

	return execution, nil
}

// FindAll obtiene todas las ejecuciones de tareas
func (r *TaskExecutionReadRepository) FindAll(ctx context.Context) ([]model.TaskExecution, error) {
	query := `
		SELECT
			e.id,
			e.metadata,
			e.task_id,
			e.worker_id,
			e.status,
			e.input_args
		FROM
			task_executions e
		ORDER BY
			(e.status->>'start_time')::timestamp DESC
		LIMIT 100
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error al consultar ejecuciones: %w", err)
	}
	defer rows.Close()

	var executions []model.TaskExecution
	taskCache := make(map[uuid.UUID]model.Task)
	workerCache := make(map[uuid.UUID]model.WorkerDefinition)

	for rows.Next() {
		var execution model.TaskExecution
		var metadataBytes, statusBytes, inputArgsBytes []byte
		var taskID, workerID uuid.UUID

		err := rows.Scan(
			&execution.ID,
			&metadataBytes,
			&taskID,
			&workerID,
			&statusBytes,
			&inputArgsBytes,
		)
		if err != nil {
			return nil, fmt.Errorf("error al escanear fila de ejecución: %w", err)
		}

		// Deserializar metadata
		if err := json.Unmarshal(metadataBytes, &execution.Metadata); err != nil {
			return nil, fmt.Errorf("error al deserializar metadata: %w", err)
		}

		// Deserializar status
		if err := json.Unmarshal(statusBytes, &execution.Status); err != nil {
			return nil, fmt.Errorf("error al deserializar status: %w", err)
		}

		// Deserializar input_args
		if err := json.Unmarshal(inputArgsBytes, &execution.InputArgs); err != nil {
			return nil, fmt.Errorf("error al deserializar input_args: %w", err)
		}

		// Buscar la tarea en el cache o cargarla
		task, exists := taskCache[taskID]
		if !exists {
			task, err = r.getTaskByID(ctx, model.AggregateID(taskID))
			if err != nil {
				return nil, err
			}
			taskCache[taskID] = task
		}
		execution.Task = task

		// Buscar el worker en el cache o cargarlo
		worker, exists := workerCache[workerID]
		if !exists {
			worker, err = r.getWorkerByID(ctx, model.AggregateID(workerID))
			if err != nil {
				return nil, err
			}
			workerCache[workerID] = worker
		}
		execution.WorkerDef = worker

		executions = append(executions, execution)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar sobre filas de ejecuciones: %w", err)
	}

	return executions, nil
}

// Count cuenta el número total de ejecuciones
func (r *TaskExecutionReadRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	query := "SELECT COUNT(*) FROM task_executions"

	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error al contar ejecuciones: %w", err)
	}

	return count, nil
}

// Exists verifica si existe una ejecución por su ID
func (r *TaskExecutionReadRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM task_executions WHERE id = $1)"

	err := r.db.QueryRowContext(ctx, query, uuid.UUID(id)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error al verificar existencia de ejecución: %w", err)
	}

	return exists, nil
}

// FindByCriteria busca ejecuciones según criterios especificados
func (r *TaskExecutionReadRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[model.TaskExecution], error) {
	// Construir la consulta base
	baseQuery := `
		SELECT
			e.id,
			e.metadata,
			e.task_id,
			e.worker_id,
			e.status,
			e.input_args
		FROM
			task_executions e
		WHERE 1=1
	`

	// Construir la consulta de conteo
	countQuery := "SELECT COUNT(*) FROM task_executions e WHERE 1=1"

	// Valores para los parámetros
	params := []interface{}{}
	paramIndex := 1

	// Aplicar filtros
	filterQuery := ""
	for key, value := range criteria.Filters {
		switch key {
		case "task_id":
			filterQuery += fmt.Sprintf(" AND e.task_id = $%d", paramIndex)
			taskID, err := uuid.Parse(value.(string))
			if err != nil {
				return ports.SearchResult[model.TaskExecution]{}, fmt.Errorf("ID de tarea inválido: %w", err)
			}
			params = append(params, taskID)
			paramIndex++
		case "worker_id":
			filterQuery += fmt.Sprintf(" AND e.worker_id = $%d", paramIndex)
			workerID, err := uuid.Parse(value.(string))
			if err != nil {
				return ports.SearchResult[model.TaskExecution]{}, fmt.Errorf("ID de worker inválido: %w", err)
			}
			params = append(params, workerID)
			paramIndex++
		case "state":
			filterQuery += fmt.Sprintf(" AND status->>'state' = $%d", paramIndex)
			params = append(params, value)
			paramIndex++
		case "start_date":
			filterQuery += fmt.Sprintf(" AND (status->>'start_time')::timestamp >= $%d", paramIndex)
			params = append(params, value)
			paramIndex++
		case "end_date":
			filterQuery += fmt.Sprintf(" AND (status->>'end_time')::timestamp <= $%d", paramIndex)
			params = append(params, value)
			paramIndex++
		}
	}

	baseQuery += filterQuery
	countQuery += filterQuery

	// Obtener el recuento total
	var totalElements int64
	err := r.db.QueryRowContext(ctx, countQuery, params...).Scan(&totalElements)
	if err != nil {
		return ports.SearchResult[model.TaskExecution]{}, fmt.Errorf("error al contar resultados: %w", err)
	}

	// Aplicar ordenación
	if criteria.SortBy != "" {
		direction := "ASC"
		if criteria.SortOrder == "desc" {
			direction = "DESC"
		}

		switch criteria.SortBy {
		case "start_time":
			baseQuery += fmt.Sprintf(" ORDER BY (status->>'start_time')::timestamp %s", direction)
		case "end_time":
			baseQuery += fmt.Sprintf(" ORDER BY (status->>'end_time')::timestamp %s", direction)
		case "name":
			baseQuery += fmt.Sprintf(" ORDER BY (metadata->>'name') %s", direction)
		default:
			baseQuery += fmt.Sprintf(" ORDER BY (status->>'start_time')::timestamp %s", direction)
		}
	} else {
		baseQuery += " ORDER BY (status->>'start_time')::timestamp DESC"
	}

	// Aplicar paginación
	if criteria.Size > 0 {
		baseQuery += fmt.Sprintf(" LIMIT $%d", paramIndex)
		params = append(params, criteria.Size)
		paramIndex++

		offset := criteria.Page * criteria.Size
		baseQuery += fmt.Sprintf(" OFFSET $%d", paramIndex)
		params = append(params, offset)
	}

	// Ejecutar la consulta paginada
	rows, err := r.db.QueryContext(ctx, baseQuery, params...)
	if err != nil {
		return ports.SearchResult[model.TaskExecution]{}, fmt.Errorf("error al consultar ejecuciones: %w", err)
	}
	defer rows.Close()

	var executions []model.TaskExecution
	taskCache := make(map[uuid.UUID]model.Task)
	workerCache := make(map[uuid.UUID]model.WorkerDefinition)

	for rows.Next() {
		var execution model.TaskExecution
		var metadataBytes, statusBytes, inputArgsBytes []byte
		var taskID, workerID uuid.UUID

		err := rows.Scan(
			&execution.ID,
			&metadataBytes,
			&taskID,
			&workerID,
			&statusBytes,
			&inputArgsBytes,
		)
		if err != nil {
			return ports.SearchResult[model.TaskExecution]{}, fmt.Errorf("error al escanear fila de ejecución: %w", err)
		}

		// Deserializar metadata
		if err := json.Unmarshal(metadataBytes, &execution.Metadata); err != nil {
			return ports.SearchResult[model.TaskExecution]{}, fmt.Errorf("error al deserializar metadata: %w", err)
		}

		// Deserializar status
		if err := json.Unmarshal(statusBytes, &execution.Status); err != nil {
			return ports.SearchResult[model.TaskExecution]{}, fmt.Errorf("error al deserializar status: %w", err)
		}

		// Deserializar input_args
		if err := json.Unmarshal(inputArgsBytes, &execution.InputArgs); err != nil {
			return ports.SearchResult[model.TaskExecution]{}, fmt.Errorf("error al deserializar input_args: %w", err)
		}

		// Buscar la tarea en el cache o cargarla
		task, exists := taskCache[taskID]
		if !exists {
			task, err = r.getTaskByID(ctx, model.AggregateID(taskID))
			if err != nil {
				return ports.SearchResult[model.TaskExecution]{}, err
			}
			taskCache[taskID] = task
		}
		execution.Task = task

		// Buscar el worker en el cache o cargarlo
		worker, exists := workerCache[workerID]
		if !exists {
			worker, err = r.getWorkerByID(ctx, model.AggregateID(workerID))
			if err != nil {
				return ports.SearchResult[model.TaskExecution]{}, err
			}
			workerCache[workerID] = worker
		}
		execution.WorkerDef = worker

		executions = append(executions, execution)
	}

	if err = rows.Err(); err != nil {
		return ports.SearchResult[model.TaskExecution]{}, fmt.Errorf("error al iterar sobre filas de ejecuciones: %w", err)
	}

	// Calcular información de paginación
	totalPages := 0
	if criteria.Size > 0 {
		totalPages = int((totalElements + int64(criteria.Size) - 1) / int64(criteria.Size))
	}

	hasNext := false
	if criteria.Size > 0 {
		hasNext = (criteria.Page + 1) < totalPages
	}

	hasPreview := criteria.Page > 0

	// Construir resultado paginado
	result := ports.SearchResult[model.TaskExecution]{
		Content:       executions,
		TotalElements: totalElements,
		TotalPages:    totalPages,
		Page:          criteria.Page,
		Size:          criteria.Size,
		HasNext:       hasNext,
		HasPrevious:   hasPreview,
	}

	return result, nil
}

// getWorkerByID obtiene un worker por su ID
func (r *TaskExecutionReadRepository) getWorkerByID(ctx context.Context, id model.AggregateID) (model.WorkerDefinition, error) {
	query := `
		SELECT
			id,
			metadata,
			spec,
			status
		FROM
			workers
		WHERE
			id = $1
	`

	var worker model.WorkerDefinition
	var metadataBytes, specBytes, statusBytes []byte

	err := r.db.QueryRowContext(ctx, query, uuid.UUID(id)).Scan(
		&worker.ID,
		&metadataBytes,
		&specBytes,
		&statusBytes,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return model.WorkerDefinition{}, fmt.Errorf("worker con ID %s no encontrado", id)
		}
		return model.WorkerDefinition{}, fmt.Errorf("error al buscar worker: %w", err)
	}

	// Deserializar metadata
	if err := json.Unmarshal(metadataBytes, &worker.Metadata); err != nil {
		return model.WorkerDefinition{}, fmt.Errorf("error al deserializar metadata del worker: %w", err)
	}

	// Deserializar spec
	if err := json.Unmarshal(specBytes, &worker.Spec); err != nil {
		return model.WorkerDefinition{}, fmt.Errorf("error al deserializar spec del worker: %w", err)
	}

	// Deserializar status
	if err := json.Unmarshal(statusBytes, &worker.Status); err != nil {
		return model.WorkerDefinition{}, fmt.Errorf("error al deserializar status del worker: %w", err)
	}

	return worker, nil
}

// getTaskByID obtiene una tarea por su ID
func (r *TaskExecutionReadRepository) getTaskByID(ctx context.Context, id model.AggregateID) (model.Task, error) {
	query := `
		SELECT
			id,
			metadata,
			spec,
			command,
			args,
			env_vars
		FROM
			tasks
		WHERE
			id = $1
	`

	var task model.Task
	var metadataBytes, specBytes, commandBytes, argsBytes, envVarsBytes []byte
	err := r.db.QueryRowContext(ctx, query, uuid.UUID(id)).Scan(
		&task.ID,
		&metadataBytes,
		&specBytes,
		&commandBytes,
		&argsBytes,
		&envVarsBytes,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return model.Task{}, fmt.Errorf("tarea con ID %s no encontrada", id)
		}
		return model.Task{}, fmt.Errorf("error al buscar tarea: %w", err)
	}

	// Deserializar metadata
	if err := json.Unmarshal(metadataBytes, &task.Metadata); err != nil {
		return model.Task{}, fmt.Errorf("error al deserializar metadata de la tarea: %w", err)
	}

	// Deserializar spec
	if err := json.Unmarshal(specBytes, &task.Spec); err != nil {
		return model.Task{}, fmt.Errorf("error al deserializar spec de la tarea: %w", err)
	}

	// Deserializar command
	if err := json.Unmarshal(commandBytes, &task.Spec.Command); err != nil {
		return model.Task{}, fmt.Errorf("error al deserializar command de la tarea: %w", err)
	}

	// Deserializar args
	if err := json.Unmarshal(argsBytes, &task.Spec.Params); err != nil {
		return model.Task{}, fmt.Errorf("error al deserializar args de la tarea: %w", err)
	}

	// Deserializar env_vars
	if err := json.Unmarshal(envVarsBytes, &task.Spec.ParamValues); err != nil {
		return model.Task{}, fmt.Errorf("error al deserializar env_vars de la tarea: %w", err)
	}

	return task, nil
}
