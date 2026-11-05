package todo

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

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

// Migrate creates the todo, reminder, and tag tables.
func (h *Handler) Migrate(ctx context.Context, db *gorm.DB) error {
	return Migrate(ctx, db)
}

// Mount adds the todo routes to mux.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/todos", h.create)
	mux.HandleFunc("GET /api/todos", h.list)
	mux.HandleFunc("PATCH /api/todos/{id}", h.setDone)
	mux.HandleFunc("DELETE /api/todos/{id}", h.delete)
	mux.HandleFunc("PUT /api/todos/{id}/reminder", h.setReminder)
	mux.HandleFunc("DELETE /api/todos/{id}/reminder", h.deleteReminder)
	mux.HandleFunc("PUT /api/todos/{id}/tags", h.setTags)
	mux.HandleFunc("POST /api/tags", h.createTag)
	mux.HandleFunc("GET /api/tags", h.listTags)
	mux.HandleFunc("GET /api/tags/{id}/todos", h.listByTag)
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

func (h *Handler) setReminder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DueAt   time.Time `json:"due_at"`
		Channel string    `json:"channel"`
	}
	if err := httpx.ReadJSON(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	item, err := h.store.SetReminder(r.Context(), r.PathValue("id"), body.DueAt, body.Channel)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, item)
}

func (h *Handler) deleteReminder(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteReminder(r.Context(), r.PathValue("id")); err != nil {
		h.writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setTags(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TagIDs []string `json:"tag_ids"`
	}
	if err := httpx.ReadJSON(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	item, err := h.store.SetTags(r.Context(), r.PathValue("id"), body.TagIDs)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, item)
}

func (h *Handler) createTag(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := httpx.ReadJSON(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	item, err := h.store.CreateTag(r.Context(), body.Name)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, item)
}

func (h *Handler) listTags(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListTags(r.Context())
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"tags": items})
}

func (h *Handler) listByTag(w http.ResponseWriter, r *http.Request) {
	tag, items, err := h.store.ListByTag(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"tag": tag, "todos": items})
}

func (h *Handler) writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "todo not found")
	case errors.Is(err, ErrReminderNotFound):
		httpx.Error(w, http.StatusNotFound, "reminder not found")
	case errors.Is(err, ErrTagNotFound):
		httpx.Error(w, http.StatusNotFound, "tag not found")
	case errors.Is(err, ErrInvalidTitle):
		httpx.Error(w, http.StatusBadRequest, "title is required")
	case errors.Is(err, ErrInvalidChannel):
		httpx.Error(w, http.StatusBadRequest, "channel is required")
	case errors.Is(err, ErrInvalidDueAt):
		httpx.Error(w, http.StatusBadRequest, "due_at is required")
	case errors.Is(err, ErrInvalidTagName):
		httpx.Error(w, http.StatusBadRequest, "tag name is required")
	case errors.Is(err, ErrDuplicateTag):
		httpx.Error(w, http.StatusConflict, "tag already exists")
	default:
		log.Printf("todo: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "internal error")
	}
}
