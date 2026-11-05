package todo

import (
	"context"
	"encoding/json"
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
	db := openTestDB(t)
	handler := New(db)
	if err := handler.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DELETE FROM todos").Error; err != nil {
		t.Fatal(err)
	}
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
	db := openTestDB(t)
	handler := New(db)
	if err := handler.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.Mount(mux)
	rec := request(t, mux, http.MethodPost, "/api/todos", `{"title":"   "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
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
