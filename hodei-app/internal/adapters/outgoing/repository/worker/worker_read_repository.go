package repository

import (
	"context"
	"database/sql"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type WorkerReadRepository struct {
	db *sql.DB
}

// NewWorkerReadRepository crea una nueva instancia del repositorio de lectura para workers
func NewWorkerReadRepository(db *sql.DB) *WorkerReadRepository {
	return &WorkerReadRepository{
		db: db,
	}
}

// FindByID busca un worker por su ID
func (r *WorkerReadRepository) FindByID(ctx context.Context, id model.AggregateID) (model.WorkerDefinition, error) {
	var worker model.WorkerDefinition
	var metadataJSON, specJSON, statusJSON []byte

	query := `
  SELECT id, metadata, spec, status
  FROM workers
  WHERE id = $1
 `

	err := r.db.QueryRowContext(ctx, query, uuid.UUID(id)).Scan(
		&worker.ID, &metadataJSON, &specJSON, &statusJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return worker, fmt.Errorf("worker no encontrado con ID: %s", id)
		}
		return worker, fmt.Errorf("error al consultar worker: %w", err)
	}

	// Deserializar metadata
	if err = json.Unmarshal(metadataJSON, &worker.Metadata); err != nil {
		return worker, fmt.Errorf("error al deserializar metadata: %w", err)
	}

	// Deserializar spec
	if err = json.Unmarshal(specJSON, &worker.Spec); err != nil {
		return worker, fmt.Errorf("error al deserializar spec: %w", err)
	}

	// Deserializar status
	if err = json.Unmarshal(statusJSON, &worker.Status); err != nil {
		return worker, fmt.Errorf("error al deserializar status: %w", err)
	}

	return worker, nil
}

// FindAll recupera todos los workers
func (r *WorkerReadRepository) FindAll(ctx context.Context) ([]model.WorkerDefinition, error) {
	var workers []model.WorkerDefinition

	query := `
  SELECT id, metadata, spec, status
  FROM workers
 `

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error al consultar workers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var worker model.WorkerDefinition
		var metadataJSON, specJSON, statusJSON []byte

		if err := rows.Scan(&worker.ID, &metadataJSON, &specJSON, &statusJSON); err != nil {
			return nil, fmt.Errorf("error al escanear worker: %w", err)
		}

		// Deserializar metadata
		if err = json.Unmarshal(metadataJSON, &worker.Metadata); err != nil {
			return nil, fmt.Errorf("error al deserializar metadata: %w", err)
		}

		// Deserializar spec
		if err = json.Unmarshal(specJSON, &worker.Spec); err != nil {
			return nil, fmt.Errorf("error al deserializar spec: %w", err)
		}

		// Deserializar status
		if err = json.Unmarshal(statusJSON, &worker.Status); err != nil {
			return nil, fmt.Errorf("error al deserializar status: %w", err)
		}

		workers = append(workers, worker)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error durante la iteración de workers: %w", err)
	}

	return workers, nil
}

// Count devuelve el número total de workers
func (r *WorkerReadRepository) Count(ctx context.Context) (int64, error) {
	var count int64

	query := "SELECT COUNT(*) FROM workers"

	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error al contar workers: %w", err)
	}

	return count, nil
}

// Exists verifica si existe un worker con el ID dado
func (r *WorkerReadRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM workers WHERE id = $1)"

	err := r.db.QueryRowContext(ctx, query, uuid.UUID(id)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error al verificar existencia del worker: %w", err)
	}

	return exists, nil
}

// FindByCriteria busca workers según criterios específicos
func (r *WorkerReadRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[model.WorkerDefinition], error) {
	result := ports.SearchResult[model.WorkerDefinition]{
		Page: criteria.Page,
		Size: criteria.Size,
	}

	// Base query
	baseQuery := "FROM workers"
	countQuery := "SELECT COUNT(*) " + baseQuery

	// Construir cláusula WHERE
	whereClause, args := r.buildWhereClause(criteria.Filters)
	if whereClause != "" {
		baseQuery += " WHERE " + whereClause
		countQuery += " WHERE " + whereClause
	}

	// Obtener count total
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&result.TotalElements)
	if err != nil {
		return result, fmt.Errorf("error al contar workers con criterios: %w", err)
	}

	// Calcular paginación
	result.TotalPages = int((result.TotalElements + int64(criteria.Size) - 1) / int64(criteria.Size))
	result.HasPrevious = criteria.Page > 0
	result.HasNext = criteria.Page < result.TotalPages-1

	// Construir ORDER BY
	orderBy := " ORDER BY metadata->>'name'"
	if criteria.SortBy != "" {
		if criteria.SortBy == "name" || criteria.SortBy == "createdAt" || criteria.SortBy == "updatedAt" {
			orderBy = fmt.Sprintf(" ORDER BY metadata->>'%s'", criteria.SortBy)
		}

		if criteria.SortOrder == "desc" {
			orderBy += " DESC"
		}
	}

	// Query final con paginación
	query := fmt.Sprintf("SELECT id, metadata, spec, status %s%s LIMIT %d OFFSET %d",
		baseQuery, orderBy, criteria.Size, criteria.Page*criteria.Size)

	// Ejecutar consulta
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return result, fmt.Errorf("error al consultar workers con criterios: %w", err)
	}
	defer rows.Close()

	// Procesar resultados
	var workers []model.WorkerDefinition
	for rows.Next() {
		var worker model.WorkerDefinition
		var metadataJSON, specJSON, statusJSON []byte

		if err := rows.Scan(&worker.ID, &metadataJSON, &specJSON, &statusJSON); err != nil {
			return result, fmt.Errorf("error al escanear worker: %w", err)
		}

		// Deserializar metadata
		if err = json.Unmarshal(metadataJSON, &worker.Metadata); err != nil {
			return result, fmt.Errorf("error al deserializar metadata: %w", err)
		}

		// Deserializar spec
		if err = json.Unmarshal(specJSON, &worker.Spec); err != nil {
			return result, fmt.Errorf("error al deserializar spec: %w", err)
		}

		// Deserializar status
		if err = json.Unmarshal(statusJSON, &worker.Status); err != nil {
			return result, fmt.Errorf("error al deserializar status: %w", err)
		}

		workers = append(workers, worker)
	}

	if err = rows.Err(); err != nil {
		return result, fmt.Errorf("error durante la iteración de workers: %w", err)
	}

	result.Content = workers
	return result, nil
}

// FindByType busca los workers según su tipo de instancia
func (r *WorkerReadRepository) FindByType(ctx context.Context, instanceType model.InstanceType) ([]model.WorkerDefinition, error) {
	var workers []model.WorkerDefinition

	query := `
  SELECT id, metadata, spec, status
  FROM workers
  WHERE spec->>'instance_type' = $1
 `

	rows, err := r.db.QueryContext(ctx, query, instanceType)
	if err != nil {
		return nil, fmt.Errorf("error al consultar workers por tipo: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var worker model.WorkerDefinition
		var metadataJSON, specJSON, statusJSON []byte

		if err := rows.Scan(&worker.ID, &metadataJSON, &specJSON, &statusJSON); err != nil {
			return nil, fmt.Errorf("error al escanear worker: %w", err)
		}

		// Deserializar metadata
		if err = json.Unmarshal(metadataJSON, &worker.Metadata); err != nil {
			return nil, fmt.Errorf("error al deserializar metadata: %w", err)
		}

		// Deserializar spec
		if err = json.Unmarshal(specJSON, &worker.Spec); err != nil {
			return nil, fmt.Errorf("error al deserializar spec: %w", err)
		}

		// Deserializar status
		if err = json.Unmarshal(statusJSON, &worker.Status); err != nil {
			return nil, fmt.Errorf("error al deserializar status: %w", err)
		}

		workers = append(workers, worker)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error durante la iteración de workers: %w", err)
	}

	return workers, nil
}

// FindByLabel busca workers que tengan una etiqueta específica
func (r *WorkerReadRepository) FindByLabel(ctx context.Context, label string) ([]model.WorkerDefinition, error) {
	var workers []model.WorkerDefinition

	query := `
  SELECT id, metadata, spec, status
  FROM workers
  WHERE metadata->'labels' ? $1
 `

	rows, err := r.db.QueryContext(ctx, query, label)
	if err != nil {
		return nil, fmt.Errorf("error al consultar workers por etiqueta: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var worker model.WorkerDefinition
		var metadataJSON, specJSON, statusJSON []byte

		if err := rows.Scan(&worker.ID, &metadataJSON, &specJSON, &statusJSON); err != nil {
			return nil, fmt.Errorf("error al escanear worker: %w", err)
		}

		// Deserializar metadata
		if err = json.Unmarshal(metadataJSON, &worker.Metadata); err != nil {
			return nil, fmt.Errorf("error al deserializar metadata: %w", err)
		}

		// Deserializar spec
		if err = json.Unmarshal(specJSON, &worker.Spec); err != nil {
			return nil, fmt.Errorf("error al deserializar spec: %w", err)
		}

		// Deserializar status
		if err = json.Unmarshal(statusJSON, &worker.Status); err != nil {
			return nil, fmt.Errorf("error al deserializar status: %w", err)
		}

		workers = append(workers, worker)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error durante la iteración de workers: %w", err)
	}

	return workers, nil
}

// buildWhereClause construye la cláusula WHERE y los argumentos correspondientes para los filtros
func (r *WorkerReadRepository) buildWhereClause(filters map[string]interface{}) (string, []interface{}) {
	var clauses []string
	var args []interface{}
	argPos := 1

	for key, value := range filters {
		switch key {
		case "name":
			clauses = append(clauses, fmt.Sprintf("metadata->>'name' ILIKE $%d", argPos))
			args = append(args, fmt.Sprintf("%%%v%%", value))
			argPos++
		case "type":
			clauses = append(clauses, fmt.Sprintf("spec->>'instance_type' = $%d", argPos))
			args = append(args, value)
			argPos++
		case "labels":
			if labels, ok := value.([]string); ok && len(labels) > 0 {
				clauses = append(clauses, fmt.Sprintf("metadata->'labels' ?| $%d", argPos))
				args = append(args, pq.Array(labels))
				argPos++
			}
		}
	}

	if len(clauses) == 0 {
		return "", args
	}

	return "(" + clauses[0] + ")", args
}
