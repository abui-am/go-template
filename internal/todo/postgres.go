package todo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// todoRow is the todos table. Reminder is one-to-one. Tags are many-to-many through todo_tags.
type todoRow struct {
	ID       string       `gorm:"primaryKey;size:32"`
	Title    string       `gorm:"size:200;not null"`
	Done     bool         `gorm:"not null"`
	Reminder *reminderRow `gorm:"foreignKey:TodoID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Tags     []tagRow     `gorm:"many2many:todo_tags;foreignKey:ID;joinForeignKey:TodoID;References:ID;joinReferences:TagID;constraint:OnDelete:CASCADE"`
}

func (todoRow) TableName() string { return "todos" }

// reminderRow is one-to-one with a todo. todo_id is the primary key, so a second reminder cannot be inserted.
type reminderRow struct {
	TodoID  string    `gorm:"primaryKey;size:32"`
	DueAt   time.Time `gorm:"not null"`
	Channel string    `gorm:"size:32;not null"`
}

func (reminderRow) TableName() string { return "reminders" }

func (row reminderRow) domain() Reminder {
	return Reminder{DueAt: row.DueAt, Channel: row.Channel}
}

// tagRow is many-to-many with todos. The pairs live in todo_tags, not on either table.
type tagRow struct {
	ID    string    `gorm:"primaryKey;size:32"`
	Name  string    `gorm:"size:40;not null;uniqueIndex"`
	Todos []todoRow `gorm:"many2many:todo_tags;foreignKey:ID;joinForeignKey:TagID;References:ID;joinReferences:TodoID"`
}

func (tagRow) TableName() string { return "tags" }

func (row tagRow) domain() Tag {
	return Tag{ID: row.ID, Name: row.Name}
}

func (row todoRow) domain() Todo {
	tags := make([]Tag, 0, len(row.Tags))
	for _, tag := range row.Tags {
		tags = append(tags, tag.domain())
	}
	var reminder *Reminder
	if row.Reminder != nil {
		value := row.Reminder.domain()
		reminder = &value
	}
	return Todo{
		ID:       row.ID,
		Title:    row.Title,
		Done:     row.Done,
		Reminder: reminder,
		Tags:     tags,
	}
}

// Migrate creates the todos, reminders, tags, and todo_tags tables.
func Migrate(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).AutoMigrate(&todoRow{}, &tagRow{}, &reminderRow{})
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
	err := p.db.WithContext(ctx).
		Preload("Reminder").
		Preload("Tags", orderByName).
		Order("id ASC").
		Find(&rows).Error
	if err != nil {
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
	return loadTodo(p.db.WithContext(ctx), id)
}

func (p *Postgres) Delete(ctx context.Context, id string) error {
	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureTodo(tx, id); err != nil {
			return err
		}
		if err := tx.Model(&todoRow{ID: id}).Association("Tags").Clear(); err != nil {
			return err
		}
		if err := tx.Delete(&reminderRow{}, "todo_id = ?", id).Error; err != nil {
			return err
		}
		result := tx.Delete(&todoRow{}, "id = ?", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (p *Postgres) SetReminder(ctx context.Context, todoID string, dueAt time.Time, channel string) (Todo, error) {
	channel, err := normalizeChannel(channel)
	if err != nil {
		return Todo{}, err
	}
	dueAt, err = normalizeDueAt(dueAt)
	if err != nil {
		return Todo{}, err
	}
	var item Todo
	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureTodo(tx, todoID); err != nil {
			return err
		}
		row := reminderRow{TodoID: todoID, DueAt: dueAt, Channel: channel}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "todo_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"due_at", "channel"}),
		}).Create(&row).Error; err != nil {
			return err
		}
		loaded, err := loadTodo(tx, todoID)
		if err != nil {
			return err
		}
		item = loaded
		return nil
	})
	if err != nil {
		return Todo{}, err
	}
	return item, nil
}

func (p *Postgres) DeleteReminder(ctx context.Context, todoID string) error {
	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureTodo(tx, todoID); err != nil {
			return err
		}
		result := tx.Delete(&reminderRow{}, "todo_id = ?", todoID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrReminderNotFound
		}
		return nil
	})
}

func (p *Postgres) CreateTag(ctx context.Context, name string) (Tag, error) {
	name, err := normalizeTagName(name)
	if err != nil {
		return Tag{}, err
	}
	id, err := newID()
	if err != nil {
		return Tag{}, err
	}
	row := tagRow{ID: id, Name: name}
	if err := p.db.WithContext(ctx).Create(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return Tag{}, ErrDuplicateTag
		}
		return Tag{}, err
	}
	return row.domain(), nil
}

func (p *Postgres) ListTags(ctx context.Context) ([]Tag, error) {
	var rows []tagRow
	if err := p.db.WithContext(ctx).Order("name ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Tag, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.domain())
	}
	return out, nil
}

func (p *Postgres) SetTags(ctx context.Context, todoID string, tagIDs []string) (Todo, error) {
	tagIDs = uniqueIDs(tagIDs)
	var item Todo
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureTodo(tx, todoID); err != nil {
			return err
		}
		var tags []tagRow
		if len(tagIDs) > 0 {
			if err := tx.Find(&tags, "id IN ?", tagIDs).Error; err != nil {
				return err
			}
			if len(tags) != len(tagIDs) {
				return ErrTagNotFound
			}
		}
		assoc := tx.Model(&todoRow{ID: todoID}).Association("Tags")
		var assocErr error
		if len(tags) == 0 {
			assocErr = assoc.Clear()
		} else {
			assocErr = assoc.Replace(tags)
		}
		if assocErr != nil {
			return assocErr
		}
		var err error
		item, err = loadTodo(tx, todoID)
		return err
	})
	if err != nil {
		return Todo{}, err
	}
	return item, nil
}

func (p *Postgres) ListByTag(ctx context.Context, tagID string) (Tag, []Todo, error) {
	var row tagRow
	err := p.db.WithContext(ctx).
		Preload("Todos", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Preload("Todos.Reminder").
		Preload("Todos.Tags", orderByName).
		First(&row, "id = ?", tagID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Tag{}, nil, ErrTagNotFound
	}
	if err != nil {
		return Tag{}, nil, err
	}
	todos := make([]Todo, 0, len(row.Todos))
	for _, todo := range row.Todos {
		todos = append(todos, todo.domain())
	}
	return row.domain(), todos, nil
}

func loadTodo(tx *gorm.DB, id string) (Todo, error) {
	var row todoRow
	err := tx.Preload("Reminder").Preload("Tags", orderByName).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Todo{}, ErrNotFound
	}
	if err != nil {
		return Todo{}, err
	}
	return row.domain(), nil
}

func ensureTodo(tx *gorm.DB, id string) error {
	var n int64
	if err := tx.Model(&todoRow{}).Where("id = ?", id).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func orderByName(db *gorm.DB) *gorm.DB {
	return db.Order("name ASC")
}

func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
