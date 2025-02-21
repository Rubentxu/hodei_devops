package store

import (
	"dev.rubentxu.devops-platform/worker/internal/ports"
	"encoding/json"
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// PocketBaseStore implementa la interfaz Store[T] usando la base de datos interna de PocketBase.
type PocketBaseStore[T any] struct {
	app       core.App
	tableName string
}

// NewPocketBaseStore crea una nueva instancia de PocketBaseStore usando PocketBase como backend.
// Se asegura de que exista una tabla con el nombre indicado que tenga dos columnas: key y value.
func NewPocketBaseStore[T any](app core.App, tableName string) (ports.Store[T], error) {
	store := &PocketBaseStore[T]{
		app:       app,
		tableName: tableName,
	}

	// Crear la tabla si no existe.
	createTableQuery := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`, tableName)

	if _, err := app.DB().NewQuery(createTableQuery).Execute(); err != nil {
		return nil, fmt.Errorf("error creando la tabla %s: %w", tableName, err)
	}

	return store, nil
}

// Put almacena o actualiza un valor en la tabla.
func (s *PocketBaseStore[T]) Put(key string, value T) error {
	dataBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("error al serializar el valor: %w", err)
	}

	// Usamos INSERT OR REPLACE para hacer un upsert.
	query := fmt.Sprintf("INSERT OR REPLACE INTO %s (key, value) VALUES ({:key}, {:value})", s.tableName)
	_, err = s.app.DB().NewQuery(query).Bind(dbx.Params{
		"key":   key,
		"value": string(dataBytes),
	}).Execute()
	if err != nil {
		return fmt.Errorf("error al insertar/actualizar el registro: %w", err)
	}
	return nil
}

// Get recupera un valor asociado a la clave.
func (s *PocketBaseStore[T]) Get(key string) (T, error) {
	var result T
	query := fmt.Sprintf("SELECT value FROM %s WHERE key = {:key} LIMIT 1", s.tableName)
	var valueStr string
	err := s.app.DB().NewQuery(query).Bind(dbx.Params{
		"key": key,
	}).One(&valueStr)
	if err != nil {
		return result, fmt.Errorf("clave no encontrada: %s", key)
	}

	if err = json.Unmarshal([]byte(valueStr), &result); err != nil {
		return result, fmt.Errorf("error al deserializar el valor: %w", err)
	}
	return result, nil
}

// List devuelve una lista de todos los valores almacenados.
func (s *PocketBaseStore[T]) List() ([]T, error) {
	var results []T
	query := fmt.Sprintf("SELECT value FROM %s", s.tableName)
	var rows []string
	if err := s.app.DB().NewQuery(query).All(&rows); err != nil {
		return nil, fmt.Errorf("error al listar registros: %w", err)
	}
	for _, row := range rows {
		var item T
		if err := json.Unmarshal([]byte(row), &item); err != nil {
			// Puedes optar por saltar registros mal formateados o retornar un error.
			continue
		}
		results = append(results, item)
	}
	return results, nil
}

// Delete elimina un registro asociado a la clave.
func (s *PocketBaseStore[T]) Delete(key string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE key = {:key}", s.tableName)
	_, err := s.app.DB().NewQuery(query).Bind(dbx.Params{
		"key": key,
	}).Execute()
	if err != nil {
		return fmt.Errorf("error al eliminar el registro: %w", err)
	}
	return nil
}

// Keys devuelve todas las claves almacenadas.
func (s *PocketBaseStore[T]) Keys() []string {
	var keys []string
	query := fmt.Sprintf("SELECT key FROM %s", s.tableName)
	if err := s.app.DB().NewQuery(query).All(&keys); err != nil {
		return nil
	}
	return keys
}
