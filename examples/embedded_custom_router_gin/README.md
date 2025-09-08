# Embedded Custom Router with Gin

This example shows how to open a custom router using the Gin framework and perform API migrations via HTTP endpoints.

What it demonstrates:
- Normal app route with Gin: GET /user
- A migration-only sub-router under /migration
  - GET /migration/user (mirrors the business handler)
  - POST /migration/up?to=N (run migrations up to N, 0 = all)
  - POST /migration/down?to=N (roll back down to N)
  - GET /migration/status (show applied versions and latest)

Migrations are taken from the existing example directory `examples/embedded/migration`.
The example uses a temporary sqlite file for migration state, so it does not modify repository files.

How to run:

```
go run ./examples/embedded_custom_router_gin
```

You should see output with the server URL and example endpoints. While the example runs (~10 seconds), you can try:

```
curl -i http://127.0.0.1:XXXXX/migration/user
curl -i -X POST http://127.0.0.1:XXXXX/migration/up?to=0
curl -i http://127.0.0.1:XXXXX/migration/status
```

Notes:
- This is a minimal example; in a real service you would bind to a fixed port with `engine.Run()` instead of using an `httptest.Server`.
- You can protect the migration routes with your own auth (e.g., middleware).
