package handlers

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"encoding/json"
	"github.com/go-playground/validator"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
)

type ResourcePoolHandler struct {
	service   ports.ResourcePoolService
	validator *validator.Validate
}

func NewResourcePoolHandler(service ports.ResourcePoolService) *ResourcePoolHandler {
	validate := validator.New()
	validate.RegisterValidation("pooltype", model.ValidatePoolType)
	return &ResourcePoolHandler{
		service:   service,
		validator: validate,
	}
}

// CreateResourcePool crea un nuevo pool de recursos
func (h *ResourcePoolHandler) CreateResourcePool(w http.ResponseWriter, r *http.Request) {
	var poolDef model.ResourcePoolDef
	if err := json.NewDecoder(r.Body).Decode(&poolDef); err != nil {
		http.Error(w, "Error al decodificar la solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(poolDef); err != nil {
		http.Error(w, "Datos de entrada inválidos: "+err.Error(), http.StatusBadRequest)
		return
	}

	createdPool, err := h.service.CreateResourcePool(r.Context(), &poolDef)
	if err != nil {
		http.Error(w, "Error al crear el pool: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdPool)
}

// GetResourcePool obtiene un pool por ID
func (h *ResourcePoolHandler) GetResourcePool(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	pool, err := h.service.GetResourcePool(r.Context(), model.AggregateID(id))
	if err != nil {
		http.Error(w, "Pool no encontrado: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pool)
}

// UpdateResourcePool actualiza un pool existente
func (h *ResourcePoolHandler) UpdateResourcePool(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var poolDef model.ResourcePoolDef
	if err := json.NewDecoder(r.Body).Decode(&poolDef); err != nil {
		http.Error(w, "Error al decodificar la solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(poolDef); err != nil {
		http.Error(w, "Datos de entrada inválidos: "+err.Error(), http.StatusBadRequest)
		return
	}

	err := h.service.UpdateResourcePool(r.Context(), model.AggregateID(id), &poolDef)
	if err != nil {
		http.Error(w, "Error al actualizar el pool: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteResourcePool elimina un pool
func (h *ResourcePoolHandler) DeleteResourcePool(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := h.service.DeleteResourcePool(r.Context(), model.AggregateID(id)); err != nil {
		http.Error(w, "Error al eliminar el pool: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListResourcePools lista los pools con criterios de búsqueda
func (h *ResourcePoolHandler) ListResourcePools(w http.ResponseWriter, r *http.Request) {
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
	}

	result, err := h.service.ListResourcePools(r.Context(), criteria)
	if err != nil {
		http.Error(w, "Error al listar los pools: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// CreateResourcePoolInstance crea una instancia de un pool
func (h *ResourcePoolHandler) CreateResourcePoolInstance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	pool, err := h.service.CreateResourcePoolInstance(r.Context(), model.AggregateID(id))
	if err != nil {
		http.Error(w, "Error al crear la instancia: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": pool.GetID()})
}

// CreateAllResourcePools crea todas las instancias de pools activos
func (h *ResourcePoolHandler) CreateAllResourcePools(w http.ResponseWriter, r *http.Request) {
	err := h.service.CreateAllResourcePools(r.Context())
	if err != nil {
		http.Error(w, "Error al crear las instancias: "+err.Error(), http.StatusInternalServerError)
		return
	}

	pools := h.service.ListActivePools()
	poolsIDs := make([]string, 0, len(pools))
	for _, p := range pools {
		poolsIDs = append(poolsIDs, p.GetID())
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Pools de recursos creados correctamente",
		"pools":   poolsIDs,
	})
}

// ListActivePools lista los pools activos
func (h *ResourcePoolHandler) ListActivePools(w http.ResponseWriter, r *http.Request) {
	pools := h.service.ListActivePools()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"activePools": pools,
	})
}

// GetActivePool obtiene un pool activo por ID
func (h *ResourcePoolHandler) GetActivePool(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	pool, exists := h.service.GetActivePool(id)
	if !exists {
		http.Error(w, "Pool activo no encontrado", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pool)
}
