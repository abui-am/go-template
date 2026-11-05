package todo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestTodoFlow(t *testing.T) {
	handler, _ := newHandler(t)
	mux := http.NewServeMux()
	handler.Mount(mux)

	create := request(t, mux, http.MethodPost, "/api/todos", `{"title":" Buy milk "}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	var created Todo
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Title != "Buy milk" || created.Done || created.ID == "" {
		t.Fatalf("%+v", created)
	}

	list := request(t, mux, http.MethodGet, "/api/todos", "")
	var listed struct {
		Todos []Todo `json:"todos"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Todos) != 1 || listed.Todos[0].ID != created.ID {
		t.Fatalf("%+v", listed.Todos)
	}

	patch := request(t, mux, http.MethodPatch, "/api/todos/"+created.ID, `{"done":true}`)
	var updated Todo
	if err := json.Unmarshal(patch.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if !updated.Done {
		t.Fatalf("%+v", updated)
	}

	deleted := request(t, mux, http.MethodDelete, "/api/todos/"+created.ID, "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", deleted.Code)
	}
	missing := request(t, mux, http.MethodDelete, "/api/todos/"+created.ID, "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d", missing.Code)
	}
}

func TestCreateRejectsBlankTitle(t *testing.T) {
	handler, _ := newHandler(t)
	mux := http.NewServeMux()
	handler.Mount(mux)
	rec := request(t, mux, http.MethodPost, "/api/todos", `{"title":"   "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestRelations(t *testing.T) {
	handler, db := newHandler(t)
	mux := http.NewServeMux()
	handler.Mount(mux)

	milk := createTodo(t, mux, "Buy milk")
	dentist := createTodo(t, mux, "Call dentist")

	firstDue := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	withReminder := request(t, mux, http.MethodPut, "/api/todos/"+milk.ID+"/reminder",
		`{"due_at":"2026-11-07T09:00:00Z","channel":" push "}`)
	if withReminder.Code != http.StatusOK {
		t.Fatalf("reminder status = %d body=%s", withReminder.Code, withReminder.Body.String())
	}
	var reminded Todo
	if err := json.Unmarshal(withReminder.Body.Bytes(), &reminded); err != nil {
		t.Fatal(err)
	}
	if reminded.Reminder == nil || reminded.Reminder.Channel != "push" || !reminded.Reminder.DueAt.Equal(firstDue) {
		t.Fatalf("%+v", reminded.Reminder)
	}

	secondDue := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	replaced := request(t, mux, http.MethodPut, "/api/todos/"+milk.ID+"/reminder",
		`{"due_at":"2026-11-08T09:00:00Z","channel":"email"}`)
	if replaced.Code != http.StatusOK {
		t.Fatalf("replace reminder status = %d body=%s", replaced.Code, replaced.Body.String())
	}
	var updated Todo
	if err := json.Unmarshal(replaced.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Reminder == nil || updated.Reminder.Channel != "email" || !updated.Reminder.DueAt.Equal(secondDue) {
		t.Fatalf("%+v", updated.Reminder)
	}
	var reminderCount int64
	if err := db.Model(&reminderRow{}).Where("todo_id = ?", milk.ID).Count(&reminderCount).Error; err != nil {
		t.Fatal(err)
	}
	if reminderCount != 1 {
		t.Fatalf("reminders = %d", reminderCount)
	}

	errands := createTag(t, mux, " errands ")
	home := createTag(t, mux, "home")
	duplicate := request(t, mux, http.MethodPost, "/api/tags", `{"name":"errands"}`)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d body=%s", duplicate.Code, duplicate.Body.String())
	}

	tagged := request(t, mux, http.MethodPut, "/api/todos/"+milk.ID+"/tags",
		fmt.Sprintf(`{"tag_ids":[%q,%q]}`, home.ID, errands.ID))
	if tagged.Code != http.StatusOK {
		t.Fatalf("tags status = %d body=%s", tagged.Code, tagged.Body.String())
	}
	var milkTagged Todo
	if err := json.Unmarshal(tagged.Body.Bytes(), &milkTagged); err != nil {
		t.Fatal(err)
	}
	if milkTagged.Reminder == nil || milkTagged.Reminder.Channel != "email" {
		t.Fatalf("reminder lost: %+v", milkTagged.Reminder)
	}
	if len(milkTagged.Tags) != 2 || milkTagged.Tags[0].Name != "errands" || milkTagged.Tags[1].Name != "home" {
		t.Fatalf("%+v", milkTagged.Tags)
	}

	shared := request(t, mux, http.MethodPut, "/api/todos/"+dentist.ID+"/tags",
		fmt.Sprintf(`{"tag_ids":[%q]}`, errands.ID))
	if shared.Code != http.StatusOK {
		t.Fatalf("share status = %d body=%s", shared.Code, shared.Body.String())
	}

	byTag := request(t, mux, http.MethodGet, "/api/tags/"+errands.ID+"/todos", "")
	var errandTodos struct {
		Tag   Tag    `json:"tag"`
		Todos []Todo `json:"todos"`
	}
	if err := json.Unmarshal(byTag.Body.Bytes(), &errandTodos); err != nil {
		t.Fatal(err)
	}
	if errandTodos.Tag.Name != "errands" || len(errandTodos.Todos) != 2 {
		t.Fatalf("%+v", errandTodos)
	}
	got := map[string]bool{}
	for _, item := range errandTodos.Todos {
		got[item.ID] = true
	}
	if !got[milk.ID] || !got[dentist.ID] {
		t.Fatalf("%+v", errandTodos.Todos)
	}

	missingTag := request(t, mux, http.MethodPut, "/api/todos/"+milk.ID+"/tags", `{"tag_ids":["missing"]}`)
	if missingTag.Code != http.StatusNotFound {
		t.Fatalf("missing tag status = %d", missingTag.Code)
	}
	var joinCount int64
	if err := db.Table("todo_tags").Where("todo_id = ?", milk.ID).Count(&joinCount).Error; err != nil {
		t.Fatal(err)
	}
	if joinCount != 2 {
		t.Fatalf("todo_tags = %d, missing tag should not change the set", joinCount)
	}

	cleared := request(t, mux, http.MethodDelete, "/api/todos/"+milk.ID+"/reminder", "")
	if cleared.Code != http.StatusNoContent {
		t.Fatalf("delete reminder status = %d body=%s", cleared.Code, cleared.Body.String())
	}
	list := request(t, mux, http.MethodGet, "/api/todos", "")
	var listed struct {
		Todos []Todo `json:"todos"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, item := range listed.Todos {
		if item.ID != milk.ID {
			continue
		}
		found = true
		if item.Reminder != nil || len(item.Tags) != 2 {
			t.Fatalf("%+v", item)
		}
	}
	if !found {
		t.Fatal("todo missing from list")
	}

	deleted := request(t, mux, http.MethodDelete, "/api/todos/"+milk.ID, "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete todo status = %d", deleted.Code)
	}
	left := request(t, mux, http.MethodGet, "/api/tags/"+errands.ID+"/todos", "")
	var remaining struct {
		Todos []Todo `json:"todos"`
	}
	if err := json.Unmarshal(left.Body.Bytes(), &remaining); err != nil {
		t.Fatal(err)
	}
	if len(remaining.Todos) != 1 || remaining.Todos[0].ID != dentist.ID {
		t.Fatalf("%+v", remaining.Todos)
	}
	var tagsLeft int64
	if err := db.Model(&tagRow{}).Where("id = ?", errands.ID).Count(&tagsLeft).Error; err != nil {
		t.Fatal(err)
	}
	if tagsLeft != 1 {
		t.Fatalf("tag should outlive the todo, count = %d", tagsLeft)
	}
}

func createTodo(t *testing.T, handler http.Handler, title string) Todo {
	t.Helper()
	rec := request(t, handler, http.MethodPost, "/api/todos", fmt.Sprintf(`{"title":%q}`, title))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create todo status = %d body=%s", rec.Code, rec.Body.String())
	}
	var item Todo
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	return item
}

func createTag(t *testing.T, handler http.Handler, name string) Tag {
	t.Helper()
	rec := request(t, handler, http.MethodPost, "/api/tags", fmt.Sprintf(`{"name":%q}`, name))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create tag status = %d body=%s", rec.Code, rec.Body.String())
	}
	var item Tag
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(name) != item.Name {
		t.Fatalf("name = %q", item.Name)
	}
	return item
}

func newHandler(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	db := openTestDB(t)
	handler := New(db)
	if err := handler.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		"DELETE FROM todo_tags",
		"DELETE FROM reminders",
		"DELETE FROM todos",
		"DELETE FROM tags",
	} {
		if err := db.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	return handler, db
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	return db
}

func request(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
