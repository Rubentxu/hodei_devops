package model

import (
	"github.com/go-playground/validator"
)

var _ AggregateRoot = (*ResourcePoolDef)(nil)

type ResourcePoolDef struct {
	ID       AggregateID        `json:"id" validate:"required"`
	Metadata Metadata           `json:"metadata" validate:"required"`
	Spec     ResourcePoolSpec   `json:"spec" validate:"required"`
	Status   ResourcePoolStatus `json:"status" validate:"required"`
}

func (r *ResourcePoolDef) GetID() AggregateID {
	return r.ID
}

type ResourcePoolSpec struct {
	PoolID       string                 `json:"poolID" validate:"required"`
	Type         string                 `json:"type" validate:"required,oneof=Kubernetes Docker VM"`
	ExtendedSpec map[string]interface{} `json:"config,omitempty" validate:"required"`
}

type ResourcePoolStatus struct {
	State string `json:"state" validate:"required,oneof=PENDING ACTIVE INACTIVE ERROR DELETED"`
}

func (r *ResourcePoolDef) Validate() error {
	validate := validator.New()

	// Registrar validación personalizada si fuera necesaria
	if err := validate.RegisterValidation("pooltype", ValidatePoolType); err != nil {
		return err
	}

	return validate.Struct(r)
}

// Función auxiliar de validación si necesitas lógica personalizada
func ValidatePoolType(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	validTypes := map[string]bool{
		"Kubernetes": true,
		"Docker":     true,
		"VM":         true,
	}
	return validTypes[value]
}
