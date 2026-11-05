package todo

import (
	"context"
	"errors"
	"log"
	"net/http"

	"go-boilerplate/internal/httpx"
	"gorm.io/gorm"
)

// Handler serves the example todo API.
type Handler struct {
	store Store
}

// NewHandler returns handlers for store.
func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// New stores todos in Postgres.
func New(db *gorm.DB) *Handler {
	return NewHandler(NewPostgres(db))
}

// Migrate creates the todos table.
func (h *Handler) Migrate(ctx context.Context, db *gorm.DB) error {
	return Migrate(ctx, db)
}

// Mount adds the todo routes to mux.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/todos", h.create)
	mux.HandleFunc("GET /api/todos", h.list)
	mux.HandleFunc("PATCH /api/todos/{id}", h.setDone)
	mux.HandleFunc("DELETE /api/todos/{id}", h.delete)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title string `json:"title"`
	}
	if err := httpx.ReadJSON(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	item, err := h.store.Create(r.Context(), body.Title)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, item)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.List(r.Context())
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"todos": items})
}

func (h *Handler) setDone(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Done bool `json:"done"`
	}
	if err := httpx.ReadJSON(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	item, err := h.store.SetDone(r.Context(), r.PathValue("id"), body.Done)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, item)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Delete(r.Context(), r.PathValue("id")); err != nil {
		h.writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "todo not found")
	case errors.Is(err, ErrInvalidTitle):
		httpx.Error(w, http.StatusBadRequest, "title is required")
	default:
		log.Printf("todo: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "internal error")
	}
}
