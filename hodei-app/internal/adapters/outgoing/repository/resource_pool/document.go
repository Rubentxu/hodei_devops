package rp_repository

import (
	"time"
)

const (
	defaultPageSize        = 10
	defaultPage            = 1
	ResourcePoolCollection = "resource_pools"
)

type ResourcePoolDocument struct {
	ID        string             `bson:"_id"`
	Metadata  ResourcePoolMeta   `bson:"metadata"`
	Spec      ResourcePoolSpec   `bson:"spec"`
	Status    ResourcePoolStatus `bson:"status"`
	Owner     string             `bson:"owner"`
	TenantID  string             `bson:"tenant_id"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

type ResourcePoolMeta struct {
	Name        string            `bson:"name"`
	Description string            `bson:"description"`
	Labels      []string          `bson:"labels"`
	Annotations map[string]string `bson:"annotations"`
	CreatedAt   time.Time         `bson:"created_at"`
	UpdatedAt   time.Time         `bson:"updated_at"`
}

type ResourcePoolSpec struct {
	PoolID string                 `bson:"pool_id"`
	Type   string                 `bson:"type"`
	Config map[string]interface{} `bson:"config"`
}

type ResourcePoolStatus struct {
	State string `bson:"state"`
}
