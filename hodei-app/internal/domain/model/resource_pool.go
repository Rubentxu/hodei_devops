package model

import (
	"errors"
	"github.com/google/uuid"
)

// ResourcePoolReadRepository implementa la interfaz ReadOnlyRepository para ResourcePoolDef
var _ AggregateRoot = (*ResourcePoolDef)(nil)

// ResourcePoolDef model
type ResourcePoolDef struct {
	ID       AggregateID
	Metadata Metadata           `db:"metadata" json:"metadata"`
	Spec     ResourcePoolSpec   `db:"spec" json:"spec"`
	Status   ResourcePoolStatus `db:"status" json:"status"`
}

func (r *ResourcePoolDef) GetID() AggregateID {
	return r.ID
}

func (id AggregateID) String() string {
	return uuid.UUID(id).String()
}

type ResourcePoolSpec struct {
	PoolID       string                 `json:"poolID"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Description  string                 `json:"description,omitempty"`
	ExtendedSpec map[string]interface{} `json:"config,omitempty"`
}

type ResourcePoolStatus struct { // Añadido ResourcePoolStatus (asumiendo que lo necesitas aunque no estaba en tu ejemplo de uso)
	State string `json:"state"` // Ejemplo de campo de estado

}

// Validate implementación de ejemplo (MOVIDO AQUI para tener el código completo en un solo lugar)
func (r *ResourcePoolDef) Validate() error {
	if r.Spec.PoolID == "" {
		return errors.New("poolID es requerido")
	}

	validTypes := map[string]bool{
		"Kubernetes": true,
		"Docker":     true,
		"VM":         true,
	}

	if !validTypes[r.Spec.Type] {
		return errors.New("tipo de recurso inválido")
	}

	return nil
}
