package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadJSON(t *testing.T) {
	var got struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Ada"}`))
	if err := ReadJSON(req, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "Ada" {
		t.Fatalf("name = %q", got.Name)
	}
}

func TestReadJSONRejectsUnknownField(t *testing.T) {
	var got struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Ada","extra":1}`))
	if err := ReadJSON(req, &got); err == nil {
		t.Fatal("expected unknown field to fail")
	}
}

func TestError(t *testing.T) {
	rec := httptest.NewRecorder()
	Error(rec, http.StatusBadRequest, "name is required")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "name is required") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
