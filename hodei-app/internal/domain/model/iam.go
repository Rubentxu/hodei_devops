package model

import (
	"errors"
	"fmt"
	"strings"
)

// RolesEj stores role definitions (by name)
// Examples of predefined roles:
//
// Admin Role:
// - Has full administrative access (admin action) to all resources
// - Uses wildcards (*) for all resource URN components to match any resource
//
// ProjectReader Role:
// - Has read-only access to all resources in the "frontend" project
// - Limited to "acme-inc" organization in "system1"
// - Uses wildcard (*) only for the resource component
//
// DatabaseAdmin Role:
// - Has write and delete permissions specifically for "database" resources
// - Can manage databases in any project within the "acme-inc" organization
// - Uses wildcard (*) for the project component
//
// Example implementation:
// var RolesEj = map[string]Role{
//	"Admin": {
//		Name: "Admin",
//		Permissions: []Permission{
//			{ResourcePattern: ResourceURN{"*", "*", "*", "*"}, AllowedActions: []Action{ActionAdmin}},
//		},
//	},
//	"ProjectReader": {
//		Name: "ProjectReader",
//		Permissions: []Permission{
//			{ResourcePattern: ResourceURN{"system1", "acme-inc", "frontend", "*"}, AllowedActions: []Action{ActionRead}},
//		},
//	},
//	"DatabaseAdmin": {
//		Name: "DatabaseAdmin",
//		Permissions: []Permission{
//			{ResourcePattern: ResourceURN{"system1", "acme-inc", "*", "database"}, AllowedActions: []Action{ActionWrite, ActionDelete}},
//		},
//	},
// }

// ResourceURN represents a unique resource identifier with a hierarchical structure.
// Format: system:organization:project:resource
// Wildcards (*) can be used in any position to match all values in that position.
type ResourceURN struct {
	System       string `json:"system" validate:"required"`       // The system identifier (e.g. "kubernetes", "aws", "gcp")
	Organization string `json:"organization" validate:"required"` // The organization identifier within the system
	Project      string `json:"project" validate:"required"`      // The project identifier within the organization
	Resource     string `json:"resource" validate:"required"`     // The specific resource identifier within the project
}

// String returns the ResourceURN as a string in the format "system:organization:project:resource".
func (r ResourceURN) String() string {
	return fmt.Sprintf("%s:%s:%s:%s", r.System, r.Organization, r.Project, r.Resource)
}

// ParseResourceURN converts an ARN-like string to a ResourceURN object.
// The string must be in the format "system:organization:project:resource".
// Returns an error if the format is invalid.
func ParseResourceURN(urn string) (ResourceURN, error) {
	parts := strings.Split(urn, ":")
	if len(parts) != 4 {
		return ResourceURN{}, fmt.Errorf("URN inválido, se esperaban 4 partes")
	}
	return ResourceURN{
		System:       parts[0],
		Organization: parts[1],
		Project:      parts[2],
		Resource:     parts[3],
	}, nil
}

// ======================
// 2. Actions and Permissions
// ======================

// Action represents a permissible action on a resource.
// Actions define the operations that can be performed on resources.
type Action string

const (
	ActionRead   Action = "read"   // Read/query operations
	ActionWrite  Action = "write"  // Create or modify operations
	ActionDelete Action = "delete" // Delete operations
	ActionAdmin  Action = "admin"  // Administrative operations (implies all other actions)
)

var (
	// Common errors for IAM operations
	ErrGroupNotFound = errors.New("group not found")
	ErrRoleNotFound  = errors.New("role not found")
)

// Permission links a resource pattern with a set of allowed actions.
// Permissions define what actions can be performed on which resources.
// Resource patterns can use concrete values or wildcards (*).
type Permission struct {
	ResourcePattern ResourceURN `json:"resourcePattern" validate:"required"`      // The resource pattern this permission applies to
	AllowedActions  []Action    `json:"allowedActions" validate:"required,min=1"` // The actions allowed on matching resources
}

// Allows checks if the specified action is allowed by this permission.
// If ActionAdmin is found in AllowedActions, any action is permitted.
func (p Permission) Allows(action Action) bool {
	if containsAction(p.AllowedActions, ActionAdmin) {
		return true
	}
	return containsAction(p.AllowedActions, action)
}

// containsAction checks if the target action exists in the actions slice.
func containsAction(actions []Action, target Action) bool {
	for _, a := range actions {
		if a == target {
			return true
		}
	}
	return false
}

// Subject is the interface implemented by User, Group, and ServiceAccount.
// It defines common methods for any entity that can be authorized
// to perform actions on resources.
type Subject interface {
	GetID() AggregateID // Returns the unique identifier of the subject
	GetRoles() []string // Returns the roles assigned to the subject
}

// User represents a human user in the system.
// Users can have directly assigned roles and can belong to groups.
type User struct {
	ID     AggregateID `json:"id" validate:"required"`      // Unique identifier
	Name   string      `json:"name" validate:"required"`    // User's name
	Roles  []string    `json:"roles" validate:"omitempty"`  // Directly assigned roles
	Groups []string    `json:"groups" validate:"omitempty"` // Names of groups the user belongs to
}

// GetID returns the user's ID.
func (u User) GetID() AggregateID {
	return u.ID
}

// GetRoles returns the roles directly assigned to the user.
func (u User) GetRoles() []string {
	return u.Roles
}

// ServiceAccount represents a service account for automation purposes.
// Service accounts are non-human subjects that can be authorized to access resources.
type ServiceAccount struct {
	ID    AggregateID `json:"id" validate:"required"`     // Unique identifier
	Name  string      `json:"name" validate:"required"`   // Service account name
	Roles []string    `json:"roles" validate:"omitempty"` // Assigned roles
}

// GetID returns the service account's ID.
func (s ServiceAccount) GetID() AggregateID {
	return s.ID
}

// GetRoles returns the roles assigned to the service account.
func (s ServiceAccount) GetRoles() []string {
	return s.Roles
}

// Group represents a collection of subjects that can be treated as a single subject.
// Groups can have roles assigned to them, and they contain members (users or other subjects).
type Group struct {
	ID           AggregateID `json:"id" validate:"required"`           // Unique identifier
	Name         string      `json:"name" validate:"required"`         // Group name
	System       string      `json:"system" validate:"required"`       // System the group belongs to
	Organization string      `json:"organization" validate:"required"` // Organization the group belongs to
	Project      string      `json:"project" validate:"omitempty"`     // Optional project scope
	Roles        []string    `json:"roles" validate:"omitempty"`       // Assigned roles
	Members      []string    `json:"members" validate:"omitempty"`     // IDs of member subjects
}

// GetID returns the group's ID.
func (g Group) GetID() AggregateID {
	return g.ID
}

// GetRoles returns the roles assigned to the group.
func (g Group) GetRoles() []string {
	return g.Roles
}

// ======================
// 4. Roles and In-memory Storage
// ======================

// Role represents a named set of permissions.
// Roles allow grouping permissions for easier management and assignment to subjects.
type Role struct {
	ID           AggregateID  `json:"id" validate:"required"`                     // Unique identifier
	Name         string       `json:"name" validate:"required"`                   // Role name
	System       string       `json:"system" validate:"required"`                 // System the role belongs to
	Organization string       `json:"organization" validate:"required"`           // Organization the role belongs to
	Project      string       `json:"project" validate:"omitempty"`               // Optional project scope
	Permissions  []Permission `json:"permissions" validate:"required,min=1,dive"` // Permissions granted by this role
}

// GetID returns the role's ID.
func (r Role) GetID() AggregateID {
	return r.ID
}

// AssignRole assigns a role to a subject.
// The implementation depends on the actual persistence mechanism.
// This example shows in-memory assignment.
func AssignRole(subject Subject, roleName string) {
	switch s := subject.(type) {
	case *User:
		if !containsString(s.Roles, roleName) {
			s.Roles = append(s.Roles, roleName)
		}
	case *ServiceAccount:
		if !containsString(s.Roles, roleName) {
			s.Roles = append(s.Roles, roleName)
		}
	default:
		// Other types can be handled as required.
	}
}

// containsString checks if the target string exists in the string slice.
func containsString(arr []string, target string) bool {
	for _, s := range arr {
		if s == target {
			return true
		}
	}
	return false
}

// AddUserToGroup adds a user to a group.
// This only updates the user's Groups field, not the group's Members field.
func AddUserToGroup(user *User, groupName string) {
	if !containsString(user.Groups, groupName) {
		user.Groups = append(user.Groups, groupName)
	}
	// Optional: update the group in groupsDB to add the member.
}
