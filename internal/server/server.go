package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"go-boilerplate/docs"
	"go-boilerplate/internal/httpx"

	httpSwagger "github.com/swaggo/http-swagger/v2"
	"gorm.io/gorm"
)

// Module mounts its HTTP routes. Add one module per resource.
type Module interface {
	Mount(mux *http.ServeMux)
}

// Migrator creates or updates a module's tables. Modules without a database can omit it.
type Migrator interface {
	Migrate(ctx context.Context, db *gorm.DB) error
}

// App is the HTTP surface. DB is nil when the process starts without DATABASE_URL.
type App struct {
	DB      *gorm.DB
	modules []Module
}

// Use migrates and mounts modules in order. This is the only registration point.
func (a *App) Use(ctx context.Context, modules ...Module) error {
	for _, module := range modules {
		if a.DB != nil {
			if migrator, ok := module.(Migrator); ok {
				if err := migrator.Migrate(ctx, a.DB); err != nil {
					return err
				}
			}
		}
		a.modules = append(a.modules, module)
	}
	return nil
}

// Handler returns the health check plus every mounted module.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /openapi.yaml", docs.Serve)
	mux.Handle("GET /swagger/", httpSwagger.Handler(httpSwagger.URL("/openapi.yaml")))
	for _, module := range a.modules {
		module.Mount(mux)
	}
	return logRequests(mux)
}

func (a *App) health(w http.ResponseWriter, _ *http.Request) {
	status := "ok"
	if a.DB != nil {
		sqlDB, err := a.DB.DB()
		if err != nil || sqlDB.Ping() != nil {
			status = "degraded"
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": status})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
