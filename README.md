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

Todos are stored in `todos`. A reminder is a separate `reminders` row whose primary key is `todo_id`, so a todo has at most one (one-to-one). Tags are shared through the `todo_tags` join table (many-to-many): a todo can have many tags, and a tag can be on many todos. The public `Todo` struct has no GORM tags.

```bash
curl -s -X PUT localhost:8080/api/todos/ID/reminder \
  -H 'content-type: application/json' \
  -d '{"due_at":"2026-11-07T09:00:00Z","channel":"push"}'
curl -s -X DELETE localhost:8080/api/todos/ID/reminder

curl -s localhost:8080/api/tags -H 'content-type: application/json' -d '{"name":"errands"}'
curl -s -X PUT localhost:8080/api/todos/ID/tags \
  -H 'content-type: application/json' \
  -d '{"tag_ids":["TAG_ID"]}'
curl -s localhost:8080/api/tags/TAG_ID/todos
```

`GET /api/todos` includes `reminder` and `tags` on each todo. Putting the reminder again updates that same row. Putting tags replaces the join rows for that todo. Deleting a todo removes its reminder and its join rows; the tag rows stay.

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
