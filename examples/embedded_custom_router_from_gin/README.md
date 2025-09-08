# Embedded Custom Router From Gin

This example shows how to reuse an existing Gin handler and expose it under a dedicated migration-only prefix using the minimal router in `pkg/router`.

- Normal business endpoint: `/user` (served by Gin)
- Migration-only mirror endpoint: `/migration/user` (served by `pkg/router`, internally forwarding to Gin)

Why: During a migration window, you might want to temporarily open a copy of an existing API under a separate prefix that can be easily closed later.

## Run

```
go run ./examples/embedded_custom_router_from_gin
```

Expected output (URLs will vary):

```
Normal API:         http://127.0.0.1:XXXXX/user
Migration mirror:   http://127.0.0.1:XXXXX/migration/user
GET /migration/user: 200
```

You can try while the example runs (~8 seconds):

```
curl -i http://127.0.0.1:XXXXX/migration/user
```

## How it works

- Define your normal Gin route: `engine.GET("/user", ...)`.
- Create a migration router: `mr := router.New(router.Options{BasePath: "/migration"})`.
- Mount the Gin handler under the migration prefix with a tiny wrapper that rewrites the path so Gin can match its original `/user` route:
  ```go
  mr.MountHandler("/user", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
      r2 := r.Clone(r.Context())
      r2.URL.Path = "/user"
      engine.ServeHTTP(w, r2)
  }))
  ```
- Attach both the normal Gin engine and the migration router to a root mux:
  ```go
  root := http.NewServeMux()
  root.Handle("/", engine) // normal routes
  root.Handle("/", mr)     // migration routes under /migration
  ```

Call `mr.Close()` to disable the migration endpoints and `mr.Open()` to re-enable them (not shown in this minimal example).
