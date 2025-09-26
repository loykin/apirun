# Embedded example with PostgreSQL store

This example shows how to run apirun programmatically (embedded) while persisting migration history in PostgreSQL instead of the default SQLite file.

## Prerequisites

- Go installed
- A running PostgreSQL instance you can connect to

You can launch a local PostgreSQL via Docker:

```bash
docker run --rm -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:16
```

## Configure connection

Set the `PG_DSN` environment variable or rely on a default:

- Default DSN (used when `PG_DSN` is not set):

```
postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
```

## Run the example

From the repository root:

```bash
# Optionally set a custom DSN
# export PG_DSN=postgres://user:pass@localhost:5432/postgres?sslmode=disable

# Run the example
go run ./examples/embedded_postgresql
```

You should see output like:

```
v001: status=200 env=map[]
migrations completed successfully (PostgreSQL store)
```

The migration history (schema_migrations, migration_runs, stored_env, and the goose version table) will be created in your PostgreSQL database.
