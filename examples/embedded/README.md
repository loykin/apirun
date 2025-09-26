# Embedded (Library) Usage Example

This example shows how to use `apirun` as a Go library inside your application (embedded mode), without invoking the CLI.

It demonstrates:
- Constructing a base `Env` and passing it to `MigrateUp`.
- Running migrations from a directory and inspecting results.

## Run

From the repository root:

```
go run ./examples/embeded
```

You should see the output from the program indicating the migration results. The included sample migration performs a simple HTTP GET to `https://httpbin.org/status/200` which returns 200.

## Files

- `main.go` – a minimal program using the public `apirun` wrapper API.
- `migration/001_check.yaml` – a simple migration with a single Up request.
