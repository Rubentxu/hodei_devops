package handlers

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"encoding/json"
	"github.com/go-playground/validator"
	"github.com/gorilla/mux"
	"net/http"
	"regexp"
	"strconv"

	"github.com/rs/zerolog/log"
)

// WorkerDefinitionHandler maneja las peticiones HTTP para WorkerDefinitions
type WorkerDefinitionHandler struct {
	service   ports.WorkerDefinitionService
	validator *validator.Validate
}

// NewWorkerDefinitionHandler crea un nuevo WorkerDefinitionHandler
func NewWorkerDefinitionHandler(service ports.WorkerDefinitionService) *WorkerDefinitionHandler {
	validate := validator.New()
	validate.RegisterValidation("memunit", validateMemoryUnit)
	validate.RegisterValidation("dir", validateDirectory)
	return &WorkerDefinitionHandler{
		service:   service,
		validator: validate,
	}
}

// validateMemoryUnit valida el formato de memoria (e.j., "1Gi", "512Mi")
func validateMemoryUnit(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	matched, _ := regexp.MatchString(`^\d+(\.\d+)?(Ki|Mi|Gi|Ti)$`, value)
	return matched
}

// validateDirectory valida que la ruta sea válida
func validateDirectory(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	return len(value) > 0 && value[0] == '/'
}

// CreateWorkerDefinition crea un nuevo WorkerDefinition
func (h *WorkerDefinitionHandler) CreateWorkerDefinition(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var workerDef model.WorkerDefinition
	if err := json.NewDecoder(r.Body).Decode(&workerDef); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error decoding request body")
		http.Error(w, "Error al decodificar la solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(workerDef); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Invalid input data")
		http.Error(w, "Datos de entrada inválidos: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the worker definition
	if err := workerDef.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Worker definition validation failed")
		http.Error(w, "Worker definition validation failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	createdWorker, err := h.service.CreateWorkerDefinition(r.Context(), &workerDef)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error al crear el worker")
		http.Error(w, "Error al crear el worker: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createdWorker); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error encoding response")
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// GetWorkerDefinition obtiene un WorkerDefinition por ID
func (h *WorkerDefinitionHandler) GetWorkerDefinition(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	worker, err := h.service.GetWorkerDefinition(r.Context(), model.AggregateID(id))
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Worker no encontrado")
		http.Error(w, "Worker no encontrado: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(worker); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error encoding response")
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// ListWorkerDefinitions lista WorkerDefinitions con criterios de búsqueda
func (h *WorkerDefinitionHandler) ListWorkerDefinitions(w http.ResponseWriter, r *http.Request) {
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
		Filters:   map[string]interface{}{}, // Puedes agregar filtros aquí si es necesario
	}

	result, err := h.service.FindWorkerDefinitions(r.Context(), criteria)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error al listar los workers")
		http.Error(w, "Error al listar los workers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error encoding response")
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// UpdateWorkerDefinition actualiza un WorkerDefinition existente
func (h *WorkerDefinitionHandler) UpdateWorkerDefinition(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	var workerDef model.WorkerDefinition
	if err := json.NewDecoder(r.Body).Decode(&workerDef); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error decoding request body")
		http.Error(w, "Error al decodificar la solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}
	workerDef.ID = model.AggregateID(id)

	if err := h.validator.Struct(workerDef); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Invalid input data")
		http.Error(w, "Datos de entrada inválidos: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the worker definition
	if err := workerDef.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Worker definition validation failed")
		http.Error(w, "Worker definition validation failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	err := h.service.UpdateWorkerDefinition(r.Context(), &workerDef)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error al actualizar el worker")
		http.Error(w, "Error al actualizar el worker: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteWorkerDefinition elimina un WorkerDefinition
func (h *WorkerDefinitionHandler) DeleteWorkerDefinition(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	err := h.service.DeleteWorkerDefinition(r.Context(), model.AggregateID(id))
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error al eliminar el worker")
		http.Error(w, "Error al eliminar el worker: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
