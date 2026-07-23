package http

import (
	"encoding/json"
	"net/http"

	"jiyi/mochat-go/internal/modules/example/application"
)

// Handler demonstrates transport-to-application dependency direction.
// It is intentionally not registered by the production server.
type Handler struct {
	service application.Service
}

func NewHandler(service application.Service) Handler {
	return Handler{service: service}
}

func (h Handler) Create(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	module, err := h.service.Create(r.Context(), request.ID, request.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id":   module.ID,
		"name": module.Name.String(),
	})
}
