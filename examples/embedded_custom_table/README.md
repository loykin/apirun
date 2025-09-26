# Embedded example with custom store table names

This example shows how to run apirun programmatically (embedded) while customizing the table/index names used by the store. This is useful when you want multiple, isolated sets of migration history in the same database file (or schema).

## What it does
- Uses the library directly (no CLI) to run versioned migrations from this example's `migration/` directory.
- Configures the store to use custom names via a single prefix (demo):
  - demo_schema_migrations
  - demo_migration_runs
  - demo_stored_env
- Uses SQLite by default; you can adapt it to PostgreSQL by setting `Backend: "postgres"` and `PostgresDSN` in `StoreOptions`.

Tip: In config.yaml, you can achieve the same by adding under `store`:

```
store:
  table_prefix: demo
```

## Run the example

From the repository root:

```bash
# Run the example
go run ./examples/embedded_custom_table
```

You should see output like:

```
v001: status=200 env=map[]
migrations completed successfully (custom table names, SQLite store)
```

The SQLite database file (apirun.db) will be created under this example's `migration` directory, but the schema tables will use the custom names.
