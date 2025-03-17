package iam

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/bson"
	"time"
)

const (
	UserCollection  = "users"
	GroupCollection = "groups"
	RoleCollection  = "roles"
)

// UserDocument represents the MongoDB document structure for User
type UserDocument struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	Roles     []string  `bson:"roles"`
	Groups    []string  `bson:"groups"`
	Password  string    `bson:"password,omitempty"`
	LastLogin time.Time `bson:"last_login,omitempty"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

// GroupDocument represents the MongoDB document structure for Group
type GroupDocument struct {
	ID           string    `bson:"_id"`
	Name         string    `bson:"name"`
	System       string    `bson:"system"`
	Organization string    `bson:"organization"`
	Project      string    `bson:"project"`
	Roles        []string  `bson:"roles"`
	Members      []string  `bson:"members"`
	CreatedAt    time.Time `bson:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"`
}

// PermissionDocument represents the MongoDB document structure for Permission
type PermissionDocument struct {
	ResourcePattern ResourceURNDocument `bson:"resource_pattern"`
	AllowedActions  []string            `bson:"allowed_actions"`
}

// ResourceURNDocument represents the MongoDB document structure for ResourceURN
type ResourceURNDocument struct {
	System       string `bson:"system"`
	Organization string `bson:"organization"`
	Project      string `bson:"project"`
	Resource     string `bson:"resource"`
}

// RoleDocument represents the MongoDB document structure for Role
type RoleDocument struct {
	ID           string               `bson:"_id"`
	Name         string               `bson:"name"`
	System       string               `bson:"system"`
	Organization string               `bson:"organization"`
	Project      string               `bson:"project"`
	Permissions  []PermissionDocument `bson:"permissions"`
	CreatedAt    time.Time            `bson:"created_at"`
	UpdatedAt    time.Time            `bson:"updated_at"`
}

// UserDocumentConverter implements the DocumentConverter interface for User
type UserDocumentConverter struct {
	generator ports.IDGenerator
}

// NewUserDocumentConverter creates a new UserDocumentConverter
func NewUserDocumentConverter(generator ports.IDGenerator) generic.DocumentConverter[*model.UserAuth, UserDocument] {
	return &UserDocumentConverter{
		generator: generator,
	}
}

// GenerateID generates a new ID
func (c *UserDocumentConverter) GenerateID() model.AggregateID {
	return c.generator.NewID()
}

// ToModel converts a MongoDB document to a domain model
func (c *UserDocumentConverter) ToModel(doc UserDocument) (*model.UserAuth, error) {
	return &model.UserAuth{
		User: model.User{
			ID:     model.AggregateID(doc.ID),
			Name:   doc.Name,
			Roles:  doc.Roles,
			Groups: doc.Groups,
		},
		Password:  doc.Password,
		LastLogin: doc.LastLogin,
		CreatedAt: doc.CreatedAt,
		UpdatedAt: doc.UpdatedAt,
	}, nil
}

// ToDocument converts a domain model to a MongoDB document
func (c *UserDocumentConverter) ToDocument(entity *model.UserAuth, ctx context.Context) UserDocument {
	if entity.ID == "" {
		entity.ID = c.GenerateID()
	}

	return UserDocument{
		ID:        entity.ID.String(),
		Name:      entity.Name,
		Roles:     entity.Roles,
		Groups:    entity.Groups,
		Password:  entity.Password,
		LastLogin: entity.LastLogin,
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}
}

// BuildFilter builds a BSON filter from search criteria
func (c *UserDocumentConverter) BuildFilter(filters map[string]interface{}) bson.M {
	bsonFilter := bson.M{}

	for key, value := range filters {
		switch key {
		case "name":
			bsonFilter["name"] = value
		case "role":
			bsonFilter["roles"] = bson.M{"$in": []string{value.(string)}}
		case "group":
			bsonFilter["groups"] = bson.M{"$in": []string{value.(string)}}
		}
	}

	return bsonFilter
}

// MapSortField maps a field name for sorting
func (c *UserDocumentConverter) MapSortField(field string) string {
	switch field {
	case "name":
		return "name"
	case "created_at":
		return "created_at"
	case "updated_at":
		return "updated_at"
	default:
		return field
	}
}

// GroupDocumentConverter implements the DocumentConverter interface for Group
type GroupDocumentConverter struct {
	generator ports.IDGenerator
}

// NewGroupDocumentConverter creates a new GroupDocumentConverter
func NewGroupDocumentConverter(generator ports.IDGenerator) generic.DocumentConverter[*model.Group, GroupDocument] {
	return &GroupDocumentConverter{
		generator: generator,
	}
}

// GenerateID generates a new ID
func (c *GroupDocumentConverter) GenerateID() model.AggregateID {
	return c.generator.NewID()
}

// ToModel converts a MongoDB document to a domain model
func (c *GroupDocumentConverter) ToModel(doc GroupDocument) (*model.Group, error) {
	return &model.Group{
		ID:           model.AggregateID(doc.ID),
		Name:         doc.Name,
		System:       doc.System,
		Organization: doc.Organization,
		Project:      doc.Project,
		Roles:        doc.Roles,
		Members:      doc.Members,
	}, nil
}

// ToDocument converts a domain model to a MongoDB document
func (c *GroupDocumentConverter) ToDocument(entity *model.Group, ctx context.Context) GroupDocument {
	if entity.ID == "" {
		entity.ID = c.GenerateID()
	}

	now := time.Now()

	return GroupDocument{
		ID:           entity.ID.String(),
		Name:         entity.Name,
		System:       entity.System,
		Organization: entity.Organization,
		Project:      entity.Project,
		Roles:        entity.Roles,
		Members:      entity.Members,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// BuildFilter builds a BSON filter from search criteria
func (c *GroupDocumentConverter) BuildFilter(filters map[string]interface{}) bson.M {
	bsonFilter := bson.M{}

	for key, value := range filters {
		switch key {
		case "name":
			bsonFilter["name"] = value
		case "system":
			bsonFilter["system"] = value
		case "organization":
			bsonFilter["organization"] = value
		case "project":
			bsonFilter["project"] = value
		case "role":
			bsonFilter["roles"] = bson.M{"$in": []string{value.(string)}}
		case "member":
			bsonFilter["members"] = bson.M{"$in": []string{value.(string)}}
		}
	}

	return bsonFilter
}

// MapSortField maps a field name for sorting
func (c *GroupDocumentConverter) MapSortField(field string) string {
	switch field {
	case "name":
		return "name"
	case "system":
		return "system"
	case "organization":
		return "organization"
	case "project":
		return "project"
	default:
		return field
	}
}

// RoleDocumentConverter implements the DocumentConverter interface for Role
type RoleDocumentConverter struct {
	generator ports.IDGenerator
}

// NewRoleDocumentConverter creates a new RoleDocumentConverter
func NewRoleDocumentConverter(generator ports.IDGenerator) generic.DocumentConverter[*model.Role, RoleDocument] {
	return &RoleDocumentConverter{
		generator: generator,
	}
}

// GenerateID generates a new ID
func (c *RoleDocumentConverter) GenerateID() model.AggregateID {
	return c.generator.NewID()
}

// ToModel converts a MongoDB document to a domain model
func (c *RoleDocumentConverter) ToModel(doc RoleDocument) (*model.Role, error) {
	permissions := make([]model.Permission, len(doc.Permissions))

	for i, permDoc := range doc.Permissions {
		actions := make([]model.Action, len(permDoc.AllowedActions))
		for j, action := range permDoc.AllowedActions {
			actions[j] = model.Action(action)
		}

		permissions[i] = model.Permission{
			ResourcePattern: model.ResourceURN{
				System:       permDoc.ResourcePattern.System,
				Organization: permDoc.ResourcePattern.Organization,
				Project:      permDoc.ResourcePattern.Project,
				Resource:     permDoc.ResourcePattern.Resource,
			},
			AllowedActions: actions,
		}
	}

	return &model.Role{
		Name:         doc.Name,
		System:       doc.System,
		Organization: doc.Organization,
		Project:      doc.Project,
		Permissions:  permissions,
	}, nil
}

// ToDocument converts a domain model to a MongoDB document
func (c *RoleDocumentConverter) ToDocument(entity *model.Role, ctx context.Context) RoleDocument {
	if entity.ID == "" {
		entity.ID = c.GenerateID()
	}

	permissions := make([]PermissionDocument, len(entity.Permissions))

	for i, perm := range entity.Permissions {
		actions := make([]string, len(perm.AllowedActions))
		for j, action := range perm.AllowedActions {
			actions[j] = string(action)
		}

		permissions[i] = PermissionDocument{
			ResourcePattern: ResourceURNDocument{
				System:       perm.ResourcePattern.System,
				Organization: perm.ResourcePattern.Organization,
				Project:      perm.ResourcePattern.Project,
				Resource:     perm.ResourcePattern.Resource,
			},
			AllowedActions: actions,
		}
	}

	now := time.Now()

	return RoleDocument{
		ID:           entity.ID.String(),
		Name:         entity.Name,
		System:       entity.System,
		Organization: entity.Organization,
		Project:      entity.Project,
		Permissions:  permissions,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// BuildFilter builds a BSON filter from search criteria
func (c *RoleDocumentConverter) BuildFilter(filters map[string]interface{}) bson.M {
	bsonFilter := bson.M{}

	for key, value := range filters {
		switch key {
		case "name":
			bsonFilter["name"] = value
		case "system":
			bsonFilter["system"] = value
		case "organization":
			bsonFilter["organization"] = value
		case "project":
			bsonFilter["project"] = value
		}
	}

	return bsonFilter
}

// MapSortField maps a field name for sorting
func (c *RoleDocumentConverter) MapSortField(field string) string {
	switch field {
	case "name":
		return "name"
	case "system":
		return "system"
	case "organization":
		return "organization"
	case "project":
		return "project"
	default:
		return field
	}
}
