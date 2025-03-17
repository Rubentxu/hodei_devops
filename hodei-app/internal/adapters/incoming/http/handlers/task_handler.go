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

// TaskHandler maneja las peticiones HTTP para Tasks
type TaskHandler struct {
	service   ports.TaskService
	validator *validator.Validate
}

// NewTaskHandler crea un nuevo TaskHandler
func NewTaskHandler(service ports.TaskService) *TaskHandler {
	validate := validator.New()
	validate.RegisterValidation("paramtype", validateParamType)
	return &TaskHandler{
		service:   service,
		validator: validate,
	}
}

// validateParamType valida que el tipo de parámetro sea uno de los permitidos
func validateParamType(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	validTypes := map[string]bool{
		"string":      true,
		"integer":     true,
		"number":      true,
		"boolean":     true,
		"select":      true,
		"multiselect": true,
		"object":      true,
		"array":       true,
		"date":        true,
		"datetime":    true,
		"file":        true,
		"password":    true,
	}
	return validTypes[value]
}

// CreateTask crea una nueva Task
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var taskDef model.Task
	if err := json.NewDecoder(r.Body).Decode(&taskDef); err != nil {
		http.Error(w, "Error al decodificar la solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(taskDef); err != nil {
		http.Error(w, "Datos de entrada inválidos: "+err.Error(), http.StatusBadRequest)
		return
	}

	createdTask, err := h.service.CreateTask(r.Context(), &taskDef)
	if err != nil {
		http.Error(w, "Error al crear la tarea: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdTask)
}

// GetTask obtiene una Task por ID
func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	task, err := h.service.GetTask(r.Context(), model.AggregateID(id))
	if err != nil {
		http.Error(w, "Tarea no encontrada: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// ListTasks lista Tasks con criterios de búsqueda
func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.ListTasks(r.Context(), criteria)
	if err != nil {
		http.Error(w, "Error al listar las tareas: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// UpdateTask actualiza una Task existente
func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var taskDef model.Task
	if err := json.NewDecoder(r.Body).Decode(&taskDef); err != nil {
		http.Error(w, "Error al decodificar la solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(taskDef); err != nil {
		http.Error(w, "Datos de entrada inválidos: "+err.Error(), http.StatusBadRequest)
		return
	}

	err := h.service.UpdateTask(r.Context(), model.AggregateID(id), &taskDef)
	if err != nil {
		http.Error(w, "Error al actualizar la tarea: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteTask elimina una Task
func (h *TaskHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	err := h.service.DeleteTask(r.Context(), model.AggregateID(id))
	if err != nil {
		http.Error(w, "Error al eliminar la tarea: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
