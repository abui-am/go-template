// Package docs serves the OpenAPI document for Swagger UI.
// Edit openapi.yaml. This file only publishes it.
package docs

import (
	"embed"
	"net/http"
)

//go:embed openapi.yaml
var spec embed.FS

// Serve writes the OpenAPI document.
func Serve(w http.ResponseWriter, _ *http.Request) {
	body, err := spec.ReadFile("openapi.yaml")
	if err != nil {
		http.Error(w, "openapi spec unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(body)
}
