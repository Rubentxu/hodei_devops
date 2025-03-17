package model

import (
	"time"
)

type AggregateID string

type AggregateRoot interface {
	GetID() AggregateID
}

func (a AggregateID) String() string {
	return string(a)
}

type Metadata struct {
	Name        string            `json:"name" validate:"required"`
	Description string            `json:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

func NewMetadata(name, description string) Metadata {
	now := time.Now().UTC()
	return Metadata{
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
		Labels:      map[string]string{},
		Annotations: map[string]string{},
	}
}

// DomainEvent es una interfaz para eventos de dominio
type DomainEvent interface {
	GetAggregateID() AggregateID
	GetEventType() string
	GetEventData() interface{}
	GetTimestamp() int64
	GetVersion() int
}
