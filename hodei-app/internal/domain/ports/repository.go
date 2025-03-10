package ports

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
)

type Repository[T model.AggregateRoot, ID model.AggregateID] interface {
	ReadOnlyRepository[T, ID]
	WriteOnlyRepository[T, ID]
}

// ReadOnlyRepository define una interfaz de solo lectura para casos donde solo necesitamos consultas
type ReadOnlyRepository[T model.AggregateRoot, ID model.AggregateID] interface {
	FindByID(ctx context.Context, id ID) (T, error)
	FindAll(ctx context.Context) ([]T, error)
	Count(ctx context.Context) (int64, error)
	Exists(ctx context.Context, id ID) (bool, error)
	FindByCriteria(ctx context.Context, criteria SearchCriteria) (SearchResult[T], error)
}

// WriteOnlyRepository define una interfaz de solo escritura
type WriteOnlyRepository[T model.AggregateRoot, ID model.AggregateID] interface {
	Save(ctx context.Context, entity T) error
	Update(ctx context.Context, entity T) error
	Delete(ctx context.Context, id ID) error
	BatchSave(ctx context.Context, entities []T) error
	BatchUpdate(ctx context.Context, entities []T) error
	BatchDelete(ctx context.Context, ids []ID) error
	WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
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
