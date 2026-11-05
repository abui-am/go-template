# Go boilerplate

Small starting point for a Go API interview. It serves HTTP with the standard library and can connect to Postgres through GORM. It has no business domain.

## Run

```bash
go test ./...
go run ./cmd/api
```

`GET /healthz` returns `{"status":"ok"}`. The process stops on Ctrl-C.

## Postgres

```bash
docker-compose up -d
export DATABASE_URL=postgres://postgres:postgres@localhost:54330/app?sslmode=disable
go run ./cmd/api
```

Leave `DATABASE_URL` unset when the task does not need a database. The API still starts.

## Add a resource

1. Put the domain types and repository in a new package under `internal/`.
2. Keep GORM tags on private row structs, not on the public API types.
3. Register those rows in `cmd/api/main.go` with `database.Migrate`.
4. Add routes on the mux inside `internal/server/server.go`.
5. Read and write JSON with `internal/httpx`.

Use `db.Transaction` when two writes must succeed together, and `clause.Locking` when a status change must allow only one winner.
