# Status (Embedded) Example

This example demonstrates how to run migrations and then query migration status (current version, applied versions, and optionally run history) from your Go program using the embedded API.

It uses the pkg/status helper for reading status and the embedded Migrator to apply migrations first so that there is actual history to display.

## Run

From the repository root:

- Without history (uses this example's migration dir by default)

  go run ./examples/status_embedded

- With history

  go run ./examples/status_embedded --history

- Specify a different migration directory

  go run ./examples/status_embedded --history --dir ./path/to/your/migration

Notes:
- The example will first execute all pending migrations in the specified directory and then print the status.
- The default uses SQLite under examples/status_embedded/migration/apirun.db and the sample migration file provided there.

## What it does
- Runs embedded migrations up to the latest for the selected directory.
- Opens the store for the directory using apirun.OpenStoreFromOptions (default SQLite).
- Retrieves status via pkg/status.FromOptions (current version, applied versions, and run history).
- Prints human-friendly output; when --history is provided, the history section is appended.
