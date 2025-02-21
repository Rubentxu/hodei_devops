package ports

type Store[T any] interface {
	Get(key string) (T, error)
	Put(key string, value T) error
	Delete(key string) error
	List() ([]T, error)
}
