package todo

import (
	"context"
	"errors"
	"strings"
)

const maxTitleLen = 200

// ErrNotFound is returned when the id does not match a todo.
var ErrNotFound = errors.New("todo not found")

// ErrInvalidTitle is returned when the title is blank or too long.
var ErrInvalidTitle = errors.New("invalid title")

// Todo is the API shape. Persistence tags stay on the private database row.
type Todo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// Store is the persistence port used by the HTTP handlers.
type Store interface {
	Create(ctx context.Context, title string) (Todo, error)
	List(ctx context.Context) ([]Todo, error)
	SetDone(ctx context.Context, id string, done bool) (Todo, error)
	Delete(ctx context.Context, id string) error
}

var _ Store = (*Postgres)(nil)

func normalizeTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > maxTitleLen {
		return "", ErrInvalidTitle
	}
	return title, nil
}
