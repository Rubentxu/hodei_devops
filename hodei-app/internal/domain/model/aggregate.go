package model

import (
	"github.com/google/uuid"
	"time"
)

type AggregateID uuid.UUID

func NewAggregateID() AggregateID {
	return AggregateID(uuid.New())
}

type AggregateRoot interface {
	GetID() AggregateID
}

type Metadata struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Labels      []string          `json:"labels,omitempty"`
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
		Labels:      []string{},
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
