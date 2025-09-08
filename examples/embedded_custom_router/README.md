# Embedded Custom Migration Router Example

This example shows how to expose an existing business API under a separate migration-only prefix so you can open it for migrations and then close it.

- Normal business endpoint: `/user`
- Migration-only mirror endpoint: `/migration/user`

The example uses the minimal router in `pkg/router` that:
- Mounts handlers under a configurable `BasePath` (default: `/migration`)
- Can be opened/closed at runtime to allow/deny access to migration endpoints

## Run

```
go run ./examples/embedded_custom_router
```

Expected output (URLs will vary):

```
Normal API:       http://127.0.0.1:XXXXX/user
Migration mirror: http://127.0.0.1:XXXXX/migration/user
GET /migration/user status before close: 200
GET /migration/user status after close:  404
GET /migration/user status after reopen: 200
Try: curl http://127.0.0.1:XXXXX/migration/user
(it will be available for ~10 seconds, then program exits)
```

You can curl the migration endpoint while the example runs:

```
curl -i http://127.0.0.1:XXXXX/migration/user
```

## How it works

- Your app defines its normal handlers on an `http.ServeMux` (e.g. `/user`).
- Create a migration router: `mr := router.New(router.Options{BasePath: "/migration"})`.
- Mount the same handler under the migration prefix: `mr.MountHandler("/user", userHandler)`.
- Attach both muxes to a root mux:
  - `root.Handle("/", appMux)`
  - `root.Handle("/", mr)`
- Call `mr.Close()` to disable the migration endpoints, `mr.Open()` to re-enable them.

This keeps your normal endpoints untouched while providing a parallel set of routes dedicated to migration tasks.
