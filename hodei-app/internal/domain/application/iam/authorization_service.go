package iam

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"fmt"
	"sync"
)

type AuthorizationService struct {
	roleRepo  ports.RoleRepository
	groupRepo ports.GroupRepository

	mu          sync.RWMutex
	rolesCache  map[string]model.Role  // clave: org:project:roleName
	groupsCache map[string]model.Group // clave: org:project:groupName
}

// NewAuthorizationService crea un nuevo AuthorizationService usando los repositorios.
func NewAuthorizationService(roleRepo ports.RoleRepository, groupRepo ports.GroupRepository) *AuthorizationService {
	return &AuthorizationService{
		roleRepo:    roleRepo,
		groupRepo:   groupRepo,
		rolesCache:  make(map[string]model.Role),
		groupsCache: make(map[string]model.Group),
	}
}

// Métodos de administración integrados:

// CreateRole crea un rol y lo agrega al repositorio y al cache.
func (a *AuthorizationService) CreateRole(role model.Role) error {
	if err := a.roleRepo.CreateRole(role); err != nil {
		return err
	}
	a.mu.Lock()
	a.rolesCache[roleKey(role.Organization, role.Project, role.Name)] = role
	a.mu.Unlock()
	return nil
}

func roleKey(org, project, roleName string) string {
	return fmt.Sprintf("%s:%s:%s", org, project, roleName)
}

func groupKey(org, project, groupName string) string {
	return fmt.Sprintf("%s:%s:%s", org, project, groupName)
}

// CreateGroup crea un grupo y lo agrega al repositorio y al cache.
func (a *AuthorizationService) CreateGroup(group model.Group) error {
	if err := a.groupRepo.CreateGroup(group); err != nil {
		return err
	}
	a.mu.Lock()
	a.groupsCache[groupKey(group.Organization, group.Project, group.Name)] = group
	a.mu.Unlock()
	return nil
}

// ListRoles retorna los roles para una organización y proyecto.
func (a *AuthorizationService) ListRoles(orgID, projectID string) ([]model.Role, error) {
	return a.roleRepo.ListRoles(orgID, projectID)
}

// ListGroups retorna los grupos para una organización y proyecto.
func (a *AuthorizationService) ListGroups(orgID, projectID string) ([]model.Group, error) {
	return a.groupRepo.ListGroups(orgID, projectID)
}

// SyncCache refresca la configuración de roles y grupos para una organización y proyecto.
func (a *AuthorizationService) SyncCache(orgID, projectID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	roles, err := a.roleRepo.ListRoles(orgID, projectID)
	if err != nil {
		return err
	}
	for _, role := range roles {
		a.rolesCache[roleKey(role.Organization, role.Project, role.Name)] = role
	}

	groups, err := a.groupRepo.ListGroups(orgID, projectID)
	if err != nil {
		return err
	}
	for _, group := range groups {
		a.groupsCache[groupKey(group.Organization, group.Project, group.Name)] = group
	}

	return nil
}

// GetRole y GetGroup permiten acceder al cache.
func (a *AuthorizationService) GetRole(orgID, projectID, roleName string) (model.Role, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	role, ok := a.rolesCache[roleKey(orgID, projectID, roleName)]
	return role, ok
}

func (a *AuthorizationService) GetGroup(orgID, projectID, groupName string) (model.Group, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	group, ok := a.groupsCache[groupKey(orgID, projectID, groupName)]
	return group, ok
}

// Authorize verifica si el subject tiene permiso para ejecutar la acción sobre el recurso,
// considerando roles directos y roles heredados a través de grupos.
// El parámetro subjectGroups representa la lista de nombres de grupos a los que pertenece el subject.
func (a *AuthorizationService) Authorize(subject model.Subject, resource model.ResourceURN, action model.Action, subjectGroups []string) bool {
	var rolesToCheck []string

	// Roles asignados directamente.
	rolesToCheck = append(rolesToCheck, subject.GetRoles()...)

	// Agregar roles obtenidos de los grupos del subject (buscando en el cache).
	for _, groupName := range subjectGroups {
		if group, exists := a.GetGroup(resource.Organization, resource.Project, groupName); exists {
			rolesToCheck = append(rolesToCheck, group.GetRoles()...)
		}
	}

	// Revisar cada rol en el cache.
	for _, roleName := range rolesToCheck {
		if role, exists := a.GetRole(resource.Organization, resource.Project, roleName); exists {
			if roleAllows(role, resource, action) {
				return true
			}
		}
	}
	return false
}

func (a *AuthorizationService) UpdateRole(role model.Role) error {
	err := a.roleRepo.UpdateRole(role)
	if err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.rolesCache[roleKey(role.Organization, role.Project, role.Name)] = role

	return nil
}

func (a *AuthorizationService) DeleteRole(orgID string, projectID string, roleName string) error {
	role, err := a.roleRepo.GetRole(orgID, projectID, roleName)
	if err != nil {
		return err
	}

	if role != nil {
		err = a.roleRepo.DeleteRole(orgID, projectID, roleName)
		if err != nil {
			return err
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.rolesCache, roleKey(role.Organization, role.Project, role.Name))

	return nil
}

func (a *AuthorizationService) DeleteGroup(orgID string, projectID string, groupName string) error {
	group, err := a.groupRepo.GetGroup(orgID, projectID, groupName)
	if err != nil {
		return err
	}

	if group != nil {
		err = a.groupRepo.DeleteGroup(orgID, projectID, groupName)
		if err != nil {
			return err
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.groupsCache, groupKey(group.Organization, group.Project, group.Name))

	return nil
}

func (a *AuthorizationService) UpdateGroup(group model.Group) error {
	err := a.groupRepo.UpdateGroup(group)
	if err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.groupsCache[groupKey(group.Organization, group.Project, group.Name)] = group

	return nil
}

func roleAllows(role model.Role, resource model.ResourceURN, action model.Action) bool {
	for _, perm := range role.Permissions {
		if matchesResourcePattern(perm.ResourcePattern, resource) && perm.Allows(action) {
			return true
		}
	}
	return false
}

func matchesResourcePattern(pattern model.ResourceURN, resource model.ResourceURN) bool {
	if pattern.System != "*" && pattern.System != resource.System {
		return false
	}
	if pattern.Organization != "*" && pattern.Organization != resource.Organization {
		return false
	}
	if pattern.Project != "*" && pattern.Project != resource.Project {
		return false
	}
	if pattern.Resource != "*" && pattern.Resource != resource.Resource {
		return false
	}
	return true
}
