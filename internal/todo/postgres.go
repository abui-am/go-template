package todo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"gorm.io/gorm"
)

type todoRow struct {
	ID    string `gorm:"primaryKey;size:32"`
	Title string `gorm:"size:200;not null"`
	Done  bool   `gorm:"not null"`
}

func (todoRow) TableName() string {
	return "todos"
}

func (row todoRow) domain() Todo {
	return Todo{ID: row.ID, Title: row.Title, Done: row.Done}
}

// Migrate creates the todos table.
func Migrate(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).AutoMigrate(&todoRow{})
}

// Postgres stores todos through GORM.
type Postgres struct {
	db *gorm.DB
}

// NewPostgres wraps an open GORM connection.
func NewPostgres(db *gorm.DB) *Postgres {
	return &Postgres{db: db}
}

func (p *Postgres) Create(ctx context.Context, title string) (Todo, error) {
	title, err := normalizeTitle(title)
	if err != nil {
		return Todo{}, err
	}
	id, err := newID()
	if err != nil {
		return Todo{}, err
	}
	row := todoRow{ID: id, Title: title}
	if err := p.db.WithContext(ctx).Create(&row).Error; err != nil {
		return Todo{}, err
	}
	return row.domain(), nil
}

func (p *Postgres) List(ctx context.Context) ([]Todo, error) {
	var rows []todoRow
	if err := p.db.WithContext(ctx).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Todo, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.domain())
	}
	return out, nil
}

func (p *Postgres) SetDone(ctx context.Context, id string, done bool) (Todo, error) {
	result := p.db.WithContext(ctx).Model(&todoRow{}).Where("id = ?", id).Updates(map[string]any{
		"done": done,
	})
	if result.Error != nil {
		return Todo{}, result.Error
	}
	if result.RowsAffected == 0 {
		return Todo{}, ErrNotFound
	}
	var row todoRow
	if err := p.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Todo{}, ErrNotFound
		}
		return Todo{}, err
	}
	return row.domain(), nil
}

func (p *Postgres) Delete(ctx context.Context, id string) error {
	result := p.db.WithContext(ctx).Delete(&todoRow{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
