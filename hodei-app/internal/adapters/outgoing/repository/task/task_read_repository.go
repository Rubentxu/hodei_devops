package repository

import (
	"context"
	"database/sql"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
)

// TaskReadRepository implementa el ReadOnlyRepository para tareas
type TaskReadRepository struct {
	db *sql.DB
}

// NewTaskReadRepository crea una nueva instancia de TaskReadRepository
func NewTaskReadRepository(db *sql.DB) *TaskReadRepository {
	return &TaskReadRepository{db: db}
}

// FindByID busca una tarea por su ID
func (r *TaskReadRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.Task, error) {
	query := `SELECT id, metadata, spec FROM tasks WHERE id = $1`

	var (
		taskID   string
		metadata []byte
		spec     []byte
	)

	err := r.db.QueryRowContext(ctx, query, id).Scan(&taskID, &metadata, &spec)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found: %w", err)
		}
		return nil, fmt.Errorf("error querying task: %w", err)
	}

	// Deserializar la tarea
	task, err := deserializeTask(taskID, metadata, spec)
	if err != nil {
		return nil, err
	}

	return task, nil
}

// FindAll devuelve todas las tareas
func (r *TaskReadRepository) FindAll(ctx context.Context) ([]*model.Task, error) {
	query := `SELECT id, metadata, spec FROM tasks`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*model.Task
	for rows.Next() {
		var (
			taskID   string
			metadata []byte
			spec     []byte
		)

		if err := rows.Scan(&taskID, &metadata, &spec); err != nil {
			return nil, fmt.Errorf("error scanning task: %w", err)
		}

		task, err := deserializeTask(taskID, metadata, spec)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tasks: %w", err)
	}

	return tasks, nil
}

// Count devuelve el número total de tareas
func (r *TaskReadRepository) Count(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM tasks`

	var count int64
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error counting tasks: %w", err)
	}

	return count, nil
}

// Exists verifica si una tarea existe
func (r *TaskReadRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM tasks WHERE id = $1)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking task existence: %w", err)
	}

	return exists, nil
}

// FindByCriteria busca tareas según los criterios dados
func (r *TaskReadRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.Task], error) {
	// Construimos la consulta base
	baseQuery := `FROM tasks WHERE 1=1`
	countQuery := `SELECT COUNT(*) ` + baseQuery
	dataQuery := `SELECT id, metadata, spec ` + baseQuery

	// Aplicamos filtros
	params := []interface{}{}
	paramCount := 1

	for key, value := range criteria.Filters {
		switch key {
		case "name":
			dataQuery += fmt.Sprintf(" AND metadata->>'name' ILIKE $%d", paramCount)
			countQuery += fmt.Sprintf(" AND metadata->>'name' ILIKE $%d", paramCount)
			params = append(params, "%"+value.(string)+"%")
			paramCount++
		case "worker_id":
			dataQuery += fmt.Sprintf(" AND spec->>'worker_id' = $%d", paramCount)
			countQuery += fmt.Sprintf(" AND spec->>'worker_id' = $%d", paramCount)
			params = append(params, value.(string))
			paramCount++
		case "tenant_id":
			dataQuery += fmt.Sprintf(" AND tenant_id = $%d", paramCount)
			countQuery += fmt.Sprintf(" AND tenant_id = $%d", paramCount)
			params = append(params, value.(string))
			paramCount++
		}
	}

	// Ordenación
	if criteria.SortBy != "" {
		sortField := criteria.SortBy
		sortOrder := "ASC"
		if criteria.SortOrder == "desc" {
			sortOrder = "DESC"
		}

		switch sortField {
		case "name":
			dataQuery += fmt.Sprintf(" ORDER BY metadata->>'name' %s", sortOrder)
		case "created_at":
			dataQuery += fmt.Sprintf(" ORDER BY created_at %s", sortOrder)
		default:
			dataQuery += " ORDER BY created_at DESC" // Ordenación por defecto
		}
	} else {
		dataQuery += " ORDER BY created_at DESC" // Ordenación por defecto
	}

	// Paginación
	if criteria.Size > 0 {
		dataQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", paramCount, paramCount+1)
		params = append(params, criteria.Size, criteria.Page*criteria.Size)
		paramCount += 2
	}

	// Ejecutar consulta de conteo
	var totalElements int64
	err := r.db.QueryRowContext(ctx, countQuery, params[:len(params)-2]...).Scan(&totalElements)
	if err != nil {
		return ports.SearchResult[*model.Task]{}, fmt.Errorf("error counting tasks: %w", err)
	}

	// Ejecutar consulta de datos
	rows, err := r.db.QueryContext(ctx, dataQuery, params...)
	if err != nil {
		return ports.SearchResult[*model.Task]{}, fmt.Errorf("error querying tasks: %w", err)
	}
	defer rows.Close()

	// Procesar resultados
	var tasks []*model.Task
	for rows.Next() {
		var (
			taskID   string
			metadata []byte
			spec     []byte
		)

		if err := rows.Scan(&taskID, &metadata, &spec); err != nil {
			return ports.SearchResult[*model.Task]{}, fmt.Errorf("error scanning task: %w", err)
		}

		task, err := deserializeTask(taskID, metadata, spec)
		if err != nil {
			return ports.SearchResult[*model.Task]{}, err
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return ports.SearchResult[*model.Task]{}, fmt.Errorf("error iterating tasks: %w", err)
	}

	// Calcular información de paginación
	var totalPages int
	if criteria.Size > 0 {
		totalPages = int((totalElements + int64(criteria.Size) - 1) / int64(criteria.Size))
	} else {
		totalPages = 1
	}

	hasNext := criteria.Page < totalPages-1
	hasPrevious := criteria.Page > 0

	// Construir resultado
	result := ports.SearchResult[*model.Task]{
		Content:       tasks,
		TotalElements: totalElements,
		TotalPages:    totalPages,
		Page:          criteria.Page,
		Size:          criteria.Size,
		HasNext:       hasNext,
		HasPrevious:   hasPrevious,
	}

	return result, nil
}

// deserializeTask convierte los datos de la base de datos a un objeto Task
func deserializeTask(taskID string, metadata []byte, spec []byte) (*model.Task, error) {
	var task model.Task

	// Convertir string ID a UUID
	id, err := uuid.Parse(taskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID format: %w", err)
	}
	task.ID = model.AggregateID(id)

	// Deserializar metadata
	if err := json.Unmarshal(metadata, &task.Metadata); err != nil {
		return nil, fmt.Errorf("error unmarshaling metadata: %w", err)
	}

	// Deserializar spec
	if err := json.Unmarshal(spec, &task.Spec); err != nil {
		return nil, fmt.Errorf("error unmarshaling spec: %w", err)
	}

	return &task, nil
}
