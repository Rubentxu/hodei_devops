package store

import (
	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
	"fmt"

	pbstore "github.com/pocketbase/pocketbase/tools/store"
)

// CacheStore implementa la interfaz Store[T] utilizando el store en memoria de PocketBase.
type CacheStore[T any] struct {
	st *pbstore.Store[string, T]
}

// NewCacheStore crea una nueva instancia de CacheStore.
func NewCacheStore[T any]() ports.Store[T] {
	return &CacheStore[T]{
		st: pbstore.New[string, T](nil),
	}
}

// Put almacena o actualiza el valor asociado a la clave.
func (s *CacheStore[T]) Put(key string, value T) error {
	s.st.Set(key, value)
	return nil
}

// Get recupera el valor asociado a la clave.
func (s *CacheStore[T]) Get(key string) (T, error) {
	val, ok := s.st.GetOk(key)
	if !ok {
		var zero T
		return zero, fmt.Errorf("clave no encontrada: %s", key)
	}
	return val, nil
}

// Delete elimina el valor asociado a la clave.
func (s *CacheStore[T]) Delete(key string) error {
	s.st.Remove(key)
	return nil
}

// List devuelve todos los valores almacenados.
func (s *CacheStore[T]) List() ([]T, error) {
	return s.st.Values(), nil
}

// Keys devuelve todas las claves almacenadas.
func (s *CacheStore[T]) Keys() []string {
	keysMap := s.st.GetAll()
	keys := make([]string, 0, len(keysMap))
	for key := range keysMap {
		keys = append(keys, key)
	}
	return keys
}
