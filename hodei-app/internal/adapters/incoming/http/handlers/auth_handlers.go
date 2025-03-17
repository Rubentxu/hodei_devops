package handlers

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/application/iam"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"encoding/json"
	"github.com/go-playground/validator"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
)

// UserHandler maneja las peticiones HTTP relacionadas con usuarios
type UserHandler struct {
	identityService *iam.IdentityService
	validator       *validator.Validate
}

// NewUserHandler crea una nueva instancia de UserHandler
func NewUserHandler(identityService *iam.IdentityService) *UserHandler {
	validate := validator.New()
	return &UserHandler{
		identityService: identityService,
		validator:       validate,
	}
}

// GetUser obtiene un usuario por ID
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	user, err := h.identityService.GetUser(model.AggregateID(id))
	if err != nil {
		http.Error(w, "Usuario no encontrado: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// ListUsers lista los usuarios con criterios de búsqueda
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	page, _ := strconv.Atoi(query.Get("page"))
	if page <= 0 {
		page = 1
	}

	size, _ := strconv.Atoi(query.Get("size"))
	if size <= 0 {
		size = 10
	}

	criteria := ports.SearchCriteria{
		Page:      page,
		Size:      size,
		SortBy:    query.Get("sortBy"),
		SortOrder: query.Get("sortOrder"),
		Filters:   map[string]interface{}{},
	}

	// Aplicar filtros adicionales si están presentes
	if org := query.Get("organization"); org != "" {
		criteria.Filters["organization"] = org
	}
	if proj := query.Get("project"); proj != "" {
		criteria.Filters["project"] = proj
	}

	// TODO: Implementar la búsqueda de usuarios con criterios
	result, err := h.identityService.ListUsers()
	if err != nil {
		http.Error(w, "Error al listar los usuarios: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// UpdateUser actualiza un usuario existente
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var userUpdate model.User

	if err := json.NewDecoder(r.Body).Decode(&userUpdate); err != nil {
		http.Error(w, "Error al decodificar la solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}
	userUpdate.ID = model.AggregateID(id)

	if err := h.validator.Struct(userUpdate); err != nil {
		http.Error(w, "Datos de entrada inválidos: "+err.Error(), http.StatusBadRequest)
		return
	}

	err := h.identityService.UpdateUser(&userUpdate)
	if err != nil {
		http.Error(w, "Error al actualizar el usuario: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteUser elimina un usuario
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	err := h.identityService.DeleteUser(model.AggregateID(id))
	if err != nil {
		http.Error(w, "Error al eliminar el usuario: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// RoleHandler maneja las peticiones HTTP relacionadas con roles
type RoleHandler struct {
	authorizationService *iam.AuthorizationService
	validator            *validator.Validate
}

// NewRoleHandler crea una nueva instancia de RoleHandler
func NewRoleHandler(authorizationService *iam.AuthorizationService) *RoleHandler {
	validate := validator.New()
	return &RoleHandler{
		authorizationService: authorizationService,
		validator:            validate,
	}
}

// CreateRole crea un nuevo rol
func (h *RoleHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var roleData model.Role
	if err := json.NewDecoder(r.Body).Decode(&roleData); err != nil {
		http.Error(w, "Error al decodificar la solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(roleData); err != nil {
		http.Error(w, "Datos de entrada inválidos: "+err.Error(), http.StatusBadRequest)
		return
	}

	err := h.authorizationService.CreateRole(roleData)
	if err != nil {
		http.Error(w, "Error al crear el rol: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	// TODO: Devolver el ID del rol creado según la especificación API REST
	//json.NewEncoder(w).Encode(map[string]string{"id": string(roleData.ID)})

}

// GetRole obtiene un rol por ID
func (h *RoleHandler) GetRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	orgID := r.URL.Query().Get("orgID")
	projectID := r.URL.Query().Get("projectID")

	role, exists := h.authorizationService.GetRole(orgID, projectID, id)
	if !exists {
		http.Error(w, "Rol no encontrado", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(role)
}

// ListRoles lista los roles con criterios de búsqueda
func (h *RoleHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	orgID := query.Get("organization")
	projectID := query.Get("project")

	result, err := h.authorizationService.ListRoles(orgID, projectID)
	if err != nil {
		http.Error(w, "Error al listar los roles: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// UpdateRole actualiza un rol existente
func (h *RoleHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var role model.Role
	if err := json.NewDecoder(r.Body).Decode(&role); err != nil {
		http.Error(w, "Error al decodificar la solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}

	role.ID = model.AggregateID(id)

	if err := h.validator.Struct(role); err != nil {
		http.Error(w, "Datos de entrada inválidos: "+err.Error(), http.StatusBadRequest)
		return
	}

	err := h.authorizationService.UpdateRole(role)
	if err != nil {
		http.Error(w, "Error al actualizar el rol: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteRole elimina un rol
func (h *RoleHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("orgID")
	projectID := r.URL.Query().Get("projectID")
	roleName := r.URL.Query().Get("roleName")

	err := h.authorizationService.DeleteRole(orgID, projectID, roleName)
	if err != nil {
		http.Error(w, "Error al eliminar el rol: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GroupHandler maneja las peticiones HTTP relacionadas con grupos
type GroupHandler struct {
	authorizationService *iam.AuthorizationService
	validator            *validator.Validate
}

// NewGroupHandler crea una nueva instancia de GroupHandler
func NewGroupHandler(authorizationService *iam.AuthorizationService) *GroupHandler {
	validate := validator.New()
	return &GroupHandler{
		authorizationService: authorizationService,
		validator:            validate,
	}
}

// CreateGroup crea un nuevo grupo
func (h *GroupHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var group model.Group
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		http.Error(w, "Error al decodificar la solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(group); err != nil {
		http.Error(w, "Datos de entrada inválidos: "+err.Error(), http.StatusBadRequest)
		return
	}

	err := h.authorizationService.CreateGroup(group)
	if err != nil {
		http.Error(w, "Error al crear el grupo: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	// TODO: Devolver el ID del grupo creado según la especificación API REST
	// json.NewEncoder(w).Encode(createdGroup)
}

// GetGroup obtiene un grupo por ID
func (h *GroupHandler) GetGroup(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("orgID")
	projectID := r.URL.Query().Get("projectID")
	roleName := r.URL.Query().Get("roleName")

	group, ok := h.authorizationService.GetGroup(orgID, projectID, roleName)
	if !ok {
		http.Error(w, "Grupo no encontrado", http.StatusNotFound)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(group)
}

// ListGroups lista los grupos con criterios de búsqueda
func (h *GroupHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("orgID")
	projectID := r.URL.Query().Get("projectID")

	result, err := h.authorizationService.ListGroups(orgID, projectID)
	if err != nil {
		http.Error(w, "Error al listar los grupos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// UpdateGroup actualiza un grupo existente
func (h *GroupHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var group model.Group
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		http.Error(w, "Error al decodificar la solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}

	group.ID = model.AggregateID(id)

	if err := h.validator.Struct(group); err != nil {
		http.Error(w, "Datos de entrada inválidos: "+err.Error(), http.StatusBadRequest)
		return
	}

	err := h.authorizationService.UpdateGroup(group)
	if err != nil {
		http.Error(w, "Error al actualizar el grupo: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteGroup elimina un grupo
func (h *GroupHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("orgID")
	projectID := r.URL.Query().Get("projectID")
	roleName := r.URL.Query().Get("roleName")

	err := h.authorizationService.DeleteGroup(orgID, projectID, roleName)
	if err != nil {
		http.Error(w, "Error al eliminar el grupo: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
