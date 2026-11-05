package server

import (
	"log"
	"net/http"
	"time"

	"go-boilerplate/internal/httpx"
	"gorm.io/gorm"
)

// App is the HTTP surface. DB is nil when the process starts without DATABASE_URL.
type App struct {
	DB *gorm.DB
}

// Handler returns the API routes.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	// Register the interview's routes on mux.
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
