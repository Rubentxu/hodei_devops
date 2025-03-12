package ports

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"errors"
)

type Repository[T model.AggregateRoot, ID model.AggregateID] interface {
	ReadOnlyRepository[T]
	WriteOnlyRepository[T]
}

// ReadOnlyRepository define una interfaz de solo lectura para casos donde solo necesitamos consultas
type ReadOnlyRepository[T model.AggregateRoot] interface {
	FindByID(ctx context.Context, id model.AggregateID) (T, error)
	FindAll(ctx context.Context) ([]T, error)
	Count(ctx context.Context) (int64, error)
	Exists(ctx context.Context, id model.AggregateID) (bool, error)
	FindByCriteria(ctx context.Context, criteria SearchCriteria) (SearchResult[T], error)
}

// WriteOnlyRepository define una interfaz de solo escritura
// Se actualiza Save para que devuelva la entidad con el ID generado por el repositorio.
type WriteOnlyRepository[T model.AggregateRoot] interface {
	Save(ctx context.Context, entity T) (T, error)
	Update(ctx context.Context, entity T) error
	Delete(ctx context.Context, id model.AggregateID) error
	BatchSave(ctx context.Context, entities []T) ([]T, error)
	BatchUpdate(ctx context.Context, entities []T) error
	BatchDelete(ctx context.Context, ids []model.AggregateID) error
}

// SearchCriteria encapsula los criterios de búsqueda comunes
type SearchCriteria struct {
	Page      int
	Size      int
	SortBy    string
	SortOrder string
	Filters   map[string]interface{}
}

// SearchResult encapsula el resultado de una búsqueda paginada
type SearchResult[T model.AggregateRoot] struct {
	Content       []T
	TotalElements int64
	TotalPages    int
	Page          int
	Size          int
	HasNext       bool
	HasPrevious   bool
}

var ( // ErrNotFound se devuelve cuando no se encuentra una entidad
	ErrNotFound = errors.New("entity not found")
)

// IDGenerator es una interfaz para generar IDs de agregados.
type IDGenerator interface {
	NewID() model.AggregateID
}
