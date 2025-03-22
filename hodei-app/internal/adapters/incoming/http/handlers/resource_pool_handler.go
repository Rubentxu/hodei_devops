package handlers

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-playground/validator"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
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

// CreateResourcePool creates a new resource pool
func (h *ResourcePoolHandler) CreateResourcePool(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var poolDef model.ResourcePoolDef
	if err := json.NewDecoder(r.Body).Decode(&poolDef); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error decoding request body")
		http.Error(w, "Error decoding request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(poolDef); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Invalid input data")
		http.Error(w, "Invalid input data: "+err.Error(), http.StatusBadRequest)
		return
	}

	createdPool, err := h.service.CreateResourcePool(ctx, &poolDef)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error creating resource pool")
		http.Error(w, "Error creating resource pool: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createdPool); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error encoding response")
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// GetResourcePool retrieves a resource pool by ID
func (h *ResourcePoolHandler) GetResourcePool(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	pool, err := h.service.GetResourcePool(ctx, model.AggregateID(id))
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Pool not found")
		http.Error(w, "Pool not found: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pool); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error encoding response")
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// UpdateResourcePool updates an existing resource pool
func (h *ResourcePoolHandler) UpdateResourcePool(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	var poolDef model.ResourcePoolDef
	if err := json.NewDecoder(r.Body).Decode(&poolDef); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error decoding request body")
		http.Error(w, "Error decoding request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(poolDef); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Invalid input data")
		http.Error(w, "Invalid input data: "+err.Error(), http.StatusBadRequest)
		return
	}

	err := h.service.UpdateResourcePool(ctx, model.AggregateID(id), &poolDef)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error updating the pool")
		http.Error(w, "Error updating the pool: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteResourcePool deletes a resource pool
func (h *ResourcePoolHandler) DeleteResourcePool(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	if err := h.service.DeleteResourcePool(ctx, model.AggregateID(id)); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error deleting the pool")
		http.Error(w, "Error deleting the pool: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListResourcePools lists resource pools with search criteria
type Metadata struct {
	Total int
	Page  int
	Size  int
}

func (h *ResourcePoolHandler) ListResourcePools(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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

	result, err := h.service.ListResourcePools(ctx, criteria)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error listing pools")
		http.Error(w, "Error listing pools: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error encoding response")
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// CreateResourcePoolInstance creates an instance of a resource pool
func (h *ResourcePoolHandler) CreateResourcePoolInstance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	pool, err := h.service.CreateResourcePoolInstance(ctx, model.AggregateID(id))
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error creating resource pool instance")
		http.Error(w, "Error creating resource pool instance: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]string{"id": pool.GetID()}); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error encoding response")
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// CreateAllResourcePools creates all instances of active pools
func (h *ResourcePoolHandler) CreateAllResourcePools(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	err := h.service.CreateAllResourcePools(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error creating all resource pool instances")
		http.Error(w, "Error creating all resource pool instances: "+err.Error(), http.StatusInternalServerError)
		return
	}

	pools := h.service.ListActivePools()
	poolsIDs := make([]string, 0, len(pools))
	for _, p := range pools {
		poolsIDs = append(poolsIDs, p.GetID())
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Pools de recursos creados correctamente",
		"pools":   poolsIDs,
	}); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error encoding response")
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// ListActivePools lists active pools
func (h *ResourcePoolHandler) ListActivePools(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pools := h.service.ListActivePools()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"activePools": pools,
	}); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error encoding response")
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// GetActivePool retrieves an active pool by ID
func (h *ResourcePoolHandler) GetActivePool(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	pool, exists := h.service.GetActivePool(id)
	if !exists {
		log.Ctx(ctx).Error().Msg("Active pool not found")
		http.Error(w, "Active pool not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pool); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error encoding response")
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}
