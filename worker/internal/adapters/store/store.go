package store

import (
	"errors"
	"sync"
)

type Store[T any] interface {
	Put(key string, value T) error
	Get(key string) (T, error)
	List() ([]T, error)
	Count() (int, error)
	Delete(key string) error
}

type InMemoryStore[T any] struct {
	data  map[string]T
	mutex sync.RWMutex
}

func NewInMemoryStore[T any]() Store[T] {
	return &InMemoryStore[T]{
		data: make(map[string]T),
	}
}

func (s *InMemoryStore[T]) Put(key string, value T) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.data[key] = value
	return nil
}

func (s *InMemoryStore[T]) Get(key string) (T, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	value, ok := s.data[key]
	if !ok {
		return *new(T), errors.New("key not found")
	}
	return value, nil
}

func (s *InMemoryStore[T]) List() ([]T, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	list := make([]T, 0, len(s.data))
	for _, value := range s.data {
		list = append(list, value)
	}
	return list, nil
}

func (s *InMemoryStore[T]) Count() (int, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.data), nil
}

func (s *InMemoryStore[T]) Delete(key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.data, key)
	return nil
}
