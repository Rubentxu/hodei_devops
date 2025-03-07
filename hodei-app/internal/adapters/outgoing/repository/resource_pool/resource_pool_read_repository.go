package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/google/uuid"
)

// ResourcePoolReadRepository implementa la interfaz ReadOnlyRepository para ResourcePoolDef
var _ ports.ReadOnlyRepository[*model.ResourcePoolDef, model.AggregateID] = (*ResourcePoolReadRepository)(nil)

type ResourcePoolReadRepository struct {
	db *sql.DB
}

// NewResourcePoolReadRepository crea una instancia del repositorio de solo lectura
func NewResourcePoolReadRepository(db *sql.DB) ports.ReadOnlyRepository[*model.ResourcePoolDef, model.AggregateID] {
	return &ResourcePoolReadRepository{
		db: db,
	}
}

// FindByID busca un ResourcePoolDef por su ID
func (r *ResourcePoolReadRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.ResourcePoolDef, error) {
	query := `
        SELECT 
            id, 
            metadata->>'name' as name,
            metadata->>'description' as description,
            metadata->>'createdAt' as created_at,
            metadata->>'updatedAt' as updated_at,
            spec->>'poolID' as pool_id,
            spec->>'name' as spec_name,
            spec->>'type' as type,
            spec->>'description' as spec_description,
            spec->>'createdAt' as spec_created_at,
            spec->>'updatedAt' as spec_updated_at,
            spec->>'config' as config,
            status->>'state' as state
        FROM resource_pools 
        WHERE id = $1
    `

	row := r.db.QueryRowContext(ctx, query, id.String())
	return r.scanResourcePool(row)
}

// FindAll retorna todos los ResourcePoolDef
func (r *ResourcePoolReadRepository) FindAll(ctx context.Context) ([]*model.ResourcePoolDef, error) {
	query := `
        SELECT 
            id, 
            metadata->>'name' as name,
            metadata->>'description' as description,
            metadata->>'createdAt' as created_at,
            metadata->>'updatedAt' as updated_at,
            spec->>'poolID' as pool_id,
            spec->>'name' as spec_name,
            spec->>'type' as type,
            spec->>'description' as spec_description,
            spec->>'createdAt' as spec_created_at,
            spec->>'updatedAt' as spec_updated_at,
            spec->>'config' as config,
            status->>'state' as state
        FROM resource_pools
    `

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error al consultar resource pools: %w", err)
	}
	defer rows.Close()

	var pools []*model.ResourcePoolDef
	for rows.Next() {
		pool, err := r.scanResourcePoolRows(rows)
		if err != nil {
			return nil, err
		}
		pools = append(pools, pool)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar sobre resultados: %w", err)
	}

	return pools, nil
}

// Count devuelve el número total de ResourcePoolDef
func (r *ResourcePoolReadRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM resource_pools").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error al contar resource pools: %w", err)
	}
	return count, nil
}

// Exists verifica si existe un ResourcePoolDef con el ID proporcionado
func (r *ResourcePoolReadRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM resource_pools WHERE id = $1)"
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error al verificar existencia: %w", err)
	}
	return exists, nil
}

// FindByCriteria busca ResourcePoolDef aplicando criterios de búsqueda y paginación
func (r *ResourcePoolReadRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.ResourcePoolDef], error) {
	// Construir consulta base con filtros
	baseQuery := `
        SELECT 
            id, 
            metadata->>'name' as name,
            metadata->>'description' as description,
            metadata->>'createdAt' as created_at,
            metadata->>'updatedAt' as updated_at,
            spec->>'poolID' as pool_id,
            spec->>'name' as spec_name,
            spec->>'type' as type,
            spec->>'description' as spec_description,
            spec->>'createdAt' as spec_created_at,
            spec->>'updatedAt' as spec_updated_at,
            spec->>'config' as config,
            status->>'state' as state
        FROM resource_pools
    `

	countQuery := "SELECT COUNT(*) FROM resource_pools"

	// Construir cláusula WHERE basada en los filtros
	whereClause, params := r.buildWhereClause(criteria.Filters)
	if whereClause != "" {
		baseQuery += " WHERE " + whereClause
		countQuery += " WHERE " + whereClause
	}

	// Añadir ordenamiento
	if criteria.SortBy != "" {
		sortOrder := "ASC"
		if strings.ToUpper(criteria.SortOrder) == "DESC" {
			sortOrder = "DESC"
		}

		// Mapear nombre de columna para ordenar (podría ser más complejo dependiendo de tus necesidades)
		sortColumn := criteria.SortBy
		switch criteria.SortBy {
		case "name":
			sortColumn = "metadata->>'name'"
		case "type":
			sortColumn = "spec->>'type'"
		case "poolID":
			sortColumn = "spec->>'poolID'"
		}

		baseQuery += fmt.Sprintf(" ORDER BY %s %s", sortColumn, sortOrder)
	}

	// Aplicar paginación
	if criteria.Size > 0 {
		offset := (criteria.Page - 1) * criteria.Size
		if offset < 0 {
			offset = 0
		}
		baseQuery += fmt.Sprintf(" LIMIT %d OFFSET %d", criteria.Size, offset)
	}

	// Ejecutar consulta para obtener total de elementos
	var totalElements int64
	err := r.db.QueryRowContext(ctx, countQuery, params...).Scan(&totalElements)
	if err != nil {
		return ports.SearchResult[*model.ResourcePoolDef]{}, fmt.Errorf("error al contar resultados filtrados: %w", err)
	}

	// Ejecutar consulta principal para obtener resultados paginados
	rows, err := r.db.QueryContext(ctx, baseQuery, params...)
	if err != nil {
		return ports.SearchResult[*model.ResourcePoolDef]{}, fmt.Errorf("error al buscar con criterios: %w", err)
	}
	defer rows.Close()

	// Construir la lista de resultados
	var content []*model.ResourcePoolDef
	for rows.Next() {
		pool, err := r.scanResourcePoolRows(rows)
		if err != nil {
			return ports.SearchResult[*model.ResourcePoolDef]{}, err
		}
		content = append(content, pool)
	}

	if err = rows.Err(); err != nil {
		return ports.SearchResult[*model.ResourcePoolDef]{}, fmt.Errorf("error al iterar sobre resultados: %w", err)
	}

	// Calcular información de paginación
	pageSize := criteria.Size
	if pageSize <= 0 {
		pageSize = 10 // valor por defecto
	}

	totalPages := int(totalElements / int64(pageSize))
	if totalElements%int64(pageSize) > 0 {
		totalPages++
	}

	currentPage := criteria.Page
	if currentPage <= 0 {
		currentPage = 1
	}

	return ports.SearchResult[*model.ResourcePoolDef]{
		Content:       content,
		TotalElements: totalElements,
		TotalPages:    totalPages,
		Page:          currentPage,
		Size:          pageSize,
		HasNext:       currentPage < totalPages,
		HasPrevious:   currentPage > 1,
	}, nil
}

// Métodos auxiliares

// buildWhereClause construye la cláusula WHERE y los parámetros para la consulta
func (r *ResourcePoolReadRepository) buildWhereClause(filters map[string]interface{}) (string, []interface{}) {
	if filters == nil || len(filters) == 0 {
		return "", nil
	}

	var conditions []string
	var params []interface{}
	paramIndex := 1

	for key, value := range filters {
		switch key {
		case "type":
			conditions = append(conditions, fmt.Sprintf("spec->>'type' = $%d", paramIndex))
			params = append(params, value)
			paramIndex++
		case "name":
			conditions = append(conditions, fmt.Sprintf("spec->>'name' = $%d", paramIndex))
			params = append(params, value)
			paramIndex++
		case "poolID":
			conditions = append(conditions, fmt.Sprintf("spec->>'poolID' = $%d", paramIndex))
			params = append(params, value)
			paramIndex++
		case "state":
			conditions = append(conditions, fmt.Sprintf("status->>'state' = $%d", paramIndex))
			params = append(params, value)
			paramIndex++
		}
	}

	return strings.Join(conditions, " AND "), params
}

// scanResourcePool escanea un registro individual en un objeto ResourcePoolDef
func (r *ResourcePoolReadRepository) scanResourcePool(row *sql.Row) (*model.ResourcePoolDef, error) {
	var pool *model.ResourcePoolDef
	var idStr, name, description, createdAt, updatedAt string
	var poolID, specName, poolType, specDescription, specCreatedAt, specUpdatedAt, config, state string

	err := row.Scan(
		&idStr,
		&name, &description, &createdAt, &updatedAt,
		&poolID, &specName, &poolType, &specDescription, &specCreatedAt, &specUpdatedAt, &config,
		&state,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return pool, fmt.Errorf("resource pool no encontrado")
		}
		return pool, fmt.Errorf("error al escanear resource pool: %w", err)
	}

	// Convertir ID a AggregateID
	id, err := uuid.Parse(idStr)
	if err != nil {
		return pool, fmt.Errorf("error al parsear ID: %w", err)
	}

	// Construir el objeto ResourcePoolDef
	pool = &model.ResourcePoolDef{
		ID: model.AggregateID(id),
		Metadata: model.Metadata{
			Name:        name,
			Description: description,
			// Aquí debes parsear createdAt y updatedAt a time.Time si es necesario
		},
		Spec: model.ResourcePoolSpec{
			PoolID:      poolID,
			Name:        specName,
			Type:        poolType,
			Description: specDescription,
			// Parsear config como JSON a ExtendedSpec si es necesario
		},
		Status: model.ResourcePoolStatus{
			State: state,
		},
	}

	return pool, nil
}

// scanResourcePoolRows escanea una fila de resultados en un objeto ResourcePoolDef
func (r *ResourcePoolReadRepository) scanResourcePoolRows(rows *sql.Rows) (*model.ResourcePoolDef, error) {
	var pool *model.ResourcePoolDef
	var idStr, name, description, createdAt, updatedAt string
	var poolID, specName, poolType, specDescription, specCreatedAt, specUpdatedAt, config, state string

	err := rows.Scan(
		&idStr,
		&name, &description, &createdAt, &updatedAt,
		&poolID, &specName, &poolType, &specDescription, &specCreatedAt, &specUpdatedAt, &config,
		&state,
	)

	if err != nil {
		return pool, fmt.Errorf("error al escanear fila: %w", err)
	}

	// Convertir ID a AggregateID
	id, err := uuid.Parse(idStr)
	if err != nil {
		return pool, fmt.Errorf("error al parsear ID: %w", err)
	}

	// Construir el objeto ResourcePoolDef
	pool = &model.ResourcePoolDef{
		ID: model.AggregateID(id),
		Metadata: model.Metadata{
			Name:        name,
			Description: description,
			// Aquí debes parsear createdAt y updatedAt a time.Time si es necesario
		},
		Spec: model.ResourcePoolSpec{
			PoolID:      poolID,
			Name:        specName,
			Type:        poolType,
			Description: specDescription,
			// Parsear config como JSON a ExtendedSpec si es necesario
		},
		Status: model.ResourcePoolStatus{
			State: state,
		},
	}

	return pool, nil
}
