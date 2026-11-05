package todo

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	maxTitleLen   = 200
	maxChannelLen = 32
	maxTagNameLen = 40
)

var (
	// ErrNotFound is returned when the id does not match a todo.
	ErrNotFound = errors.New("todo not found")
	// ErrReminderNotFound is returned when the todo has no reminder.
	ErrReminderNotFound = errors.New("reminder not found")
	// ErrTagNotFound is returned when a tag id does not exist.
	ErrTagNotFound = errors.New("tag not found")
	// ErrInvalidTitle is returned when the title is blank or too long.
	ErrInvalidTitle = errors.New("invalid title")
	// ErrInvalidChannel is returned when the reminder channel is blank or too long.
	ErrInvalidChannel = errors.New("invalid channel")
	// ErrInvalidDueAt is returned when the reminder time is missing.
	ErrInvalidDueAt = errors.New("invalid due time")
	// ErrInvalidTagName is returned when the tag name is blank or too long.
	ErrInvalidTagName = errors.New("invalid tag name")
	// ErrDuplicateTag is returned when a tag name is already taken.
	ErrDuplicateTag = errors.New("duplicate tag")
)

// Todo is the API shape. Persistence tags stay on the private database row.
// Reminder is nil when the todo has no reminder. Tags is empty when it has none.
type Todo struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Done     bool      `json:"done"`
	Reminder *Reminder `json:"reminder"`
	Tags     []Tag     `json:"tags"`
}

// Reminder is one-to-one with a todo. The todo id is the reminder's primary key.
type Reminder struct {
	DueAt   time.Time `json:"due_at"`
	Channel string    `json:"channel"`
}

// Tag is shared by many todos through the todo_tags table.
type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Store is the persistence port used by the HTTP handlers.
type Store interface {
	Create(ctx context.Context, title string) (Todo, error)
	List(ctx context.Context) ([]Todo, error)
	SetDone(ctx context.Context, id string, done bool) (Todo, error)
	Delete(ctx context.Context, id string) error
	SetReminder(ctx context.Context, todoID string, dueAt time.Time, channel string) (Todo, error)
	DeleteReminder(ctx context.Context, todoID string) error
	CreateTag(ctx context.Context, name string) (Tag, error)
	ListTags(ctx context.Context) ([]Tag, error)
	SetTags(ctx context.Context, todoID string, tagIDs []string) (Todo, error)
	ListByTag(ctx context.Context, tagID string) (Tag, []Todo, error)
}

var _ Store = (*Postgres)(nil)

func normalizeTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > maxTitleLen {
		return "", ErrInvalidTitle
	}
	return title, nil
}

func normalizeChannel(channel string) (string, error) {
	channel = strings.TrimSpace(channel)
	if channel == "" || len(channel) > maxChannelLen {
		return "", ErrInvalidChannel
	}
	return channel, nil
}

func normalizeDueAt(dueAt time.Time) (time.Time, error) {
	if dueAt.IsZero() {
		return time.Time{}, ErrInvalidDueAt
	}
	return dueAt.UTC(), nil
}

func normalizeTagName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxTagNameLen {
		return "", ErrInvalidTagName
	}
	return name, nil
}

func uniqueIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
