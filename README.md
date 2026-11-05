# Go boilerplate

Small starting point for a Go API interview. It serves HTTP with the standard library and can connect to Postgres through GORM. `internal/todo` is the working example.

## Todo example

```bash
go run ./cmd/api
```

```bash
curl -s localhost:8080/api/todos -H 'content-type: application/json' -d '{"title":"Buy milk"}'
curl -s localhost:8080/api/todos
curl -s -X PATCH localhost:8080/api/todos/ID -H 'content-type: application/json' -d '{"done":true}'
curl -s -X DELETE localhost:8080/api/todos/ID
```

Todos are stored in the `todos` table. The public `Todo` struct has no GORM tags.

## Run

```bash
docker-compose up -d
cp .env.example .env
go run ./cmd/api
```

`go run` reads `.env`. Tests still need the variable in the shell:

```bash
export DATABASE_URL=postgres://postgres:postgres@localhost:54330/app?sslmode=disable
go test ./...
```

`GET /healthz` returns `{"status":"ok"}`. Swagger UI is at [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html). Add a route by editing `docs/openapi.yaml`, then restart the server. The process stops on Ctrl-C.

## Add a resource

Copy `internal/todo`. Give the package `New(db *gorm.DB) *Handler`, `Mount`, and `Migrate`, then append it in `cmd/api/main.go`:

```go
app.Use(ctx, todo.New(app.DB), notes.New(app.DB))
```

`server` does not import the resource. `New` receives the open `*gorm.DB`, and `Migrate` creates that resource's table. Keep GORM tags on private row structs. Read and write JSON with `internal/httpx`.

Use `db.Transaction` when two writes must succeed together, and `clause.Locking` when a status change must allow only one winner.
