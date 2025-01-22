package store

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/boltdb/bolt"
)

// BoltDBStore implementa la interfaz `Store` utilizando BoltDB como backend y genéricos.
type BoltDBStore[T any] struct {
	db         *bolt.DB
	bucketName []byte
}

func (s *BoltDBStore[T]) Keys() []string {
	//TODO implement me
	panic("implement me")
}

// NewBoltDBStore crea una nueva instancia de BoltDBStore.
// `filename` es el nombre del archivo de la base de datos BoltDB.
// `mode` son los permisos del archivo (por ejemplo, 0600).
// `bucketName` es el nombre del bucket que se utilizará para almacenar los datos.
func NewBoltDBStore[T any](filename string, mode int, bucketName string) (Store[T], error) {
	db, err := bolt.Open(filename, os.FileMode(mode), &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("error al abrir la base de datos: %v", err)
	}

	// Asegurar que el bucket exista.
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("error al crear el bucket: %v", err)
	}

	return &BoltDBStore[T]{
		db:         db,
		bucketName: []byte(bucketName),
	}, nil
}

// Put almacena un valor en BoltDB.
func (s *BoltDBStore[T]) Put(key string, value T) error {
	// Serializar el valor a JSON.
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("error al serializar el valor: %v", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(s.bucketName)
		return bucket.Put([]byte(key), data)
	})
}

// Get recupera un valor de BoltDB.
func (s *BoltDBStore[T]) Get(key string) (T, error) {
	var result T
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(s.bucketName)
		data := bucket.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("clave no encontrada: %s", key)
		}
		return json.Unmarshal(data, &result)
	})
	return result, err
}

// List devuelve una lista de todos los valores en el bucket.
func (s *BoltDBStore[T]) List() ([]T, error) {
	var results []T
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(s.bucketName)
		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var result T
			if err := json.Unmarshal(v, &result); err != nil {
				return err
			}
			results = append(results, result)
		}
		return nil
	})
	return results, err
}

// Count devuelve el número de elementos en el bucket.
func (s *BoltDBStore[T]) Count() (int, error) {
	count := 0
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(s.bucketName)
		count = bucket.Stats().KeyN
		return nil
	})
	return count, err
}

// Delete elimina un elemento de BoltDB.
func (s *BoltDBStore[T]) Delete(key string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(s.bucketName)
		return bucket.Delete([]byte(key))
	})
}

// Close cierra la conexión a la base de datos BoltDB.
func (s *BoltDBStore[T]) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
