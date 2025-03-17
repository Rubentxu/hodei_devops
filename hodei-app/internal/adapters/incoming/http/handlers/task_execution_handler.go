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

type TaskExecutionHandler struct {
	service   ports.TaskExecutionService
	hodeiApp  ports.HodeiAppManager
	validator *validator.Validate
}

func NewTaskExecutionHandler(service ports.TaskExecutionService) *TaskExecutionHandler {
	return &TaskExecutionHandler{
		service:   service,
		validator: validator.New(),
	}
}

// CreateTaskExecution crea una nueva ejecución de tarea
func (h *TaskExecutionHandler) ExecuteTask(w http.ResponseWriter, r *http.Request) {
	var request model.TaskExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Error al decodificar la solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(request); err != nil {
		http.Error(w, "Datos de entrada inválidos: "+err.Error(), http.StatusBadRequest)
		return
	}
	context := r.Context()
	createdExec, err := h.hodeiApp.AddTask(request, context)
	if err != nil {
		http.Error(w, "Error al crear la ejecución: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdExec.Execution)
}

// GetTaskExecution obtiene una ejecución por ID
func (h *TaskExecutionHandler) GetTaskExecution(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	execution, err := h.service.GetTaskExecution(r.Context(), model.AggregateID(id))
	if err != nil {
		http.Error(w, "Ejecución no encontrada: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(execution)
}

// ListTaskExecutions lista las ejecuciones con criterios de búsqueda
func (h *TaskExecutionHandler) ListTaskExecutions(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.ListTaskExecutions(r.Context(), criteria)
	if err != nil {
		http.Error(w, "Error al listar las ejecuciones: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetTaskExecutionMetrics obtiene métricas de ejecución
func (h *TaskExecutionHandler) GetTaskExecutionMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.service.GetTaskExecutionMetrics(r.Context())
	if err != nil {
		http.Error(w, "Error al obtener métricas: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}
