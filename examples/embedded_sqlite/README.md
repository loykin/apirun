# Embedded example with SQLite store

This example shows how to run apirun programmatically (embedded) while persisting migration history in a local SQLite database file (the default behavior).

## Run the example

From the repository root:

```bash
# Run the example
go run ./examples/embedded_sqlite
```

You should see output like:

```
v001: status=200 env=map[]
migrations completed successfully (SQLite store)
```

The migration history (schema_migrations, migration_runs, stored_env, and the goose version table) will be created in a SQLite file named `apirun.db` under the example's `migration` directory.
