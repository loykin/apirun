# Custom Router Auth From Gin (JWT attrs.super_admin)

This example demonstrates how to:
- Register an existing Gin handler into the custom migration router (pkg/router)
- Protect the migration routes with JWT authentication middleware (internal/auth/custom_jwt)
- Issue a JWT that includes a custom payload with attrs.super_admin = true
- Authorize access based on that attribute

What it exposes:
- Normal business route (Gin): GET /user
- Migration-only mirror: GET /migration/user (served by pkg/router, forwarded to Gin)

Authorization rules:
- Requests to /migration/* must include Authorization: Bearer <token>
- The token must be valid (HS256 with the configured secret)
- The token payload must contain {"attrs": {"super_admin": true}}

## Run

```
go run ./examples/custom_router_auth_from_gin
```

While it runs (~10 seconds), try:

```
# Without token → 401
curl -i http://127.0.0.1:XXXXX/migration/user

# With token → 200
# Use the token printed by the program (shown as TOKEN=...)
curl -i -H "Authorization: Bearer <paste-your-token>" http://127.0.0.1:XXXXX/migration/user
```

## How it works
- Gin defines GET /user.
- We add a small bridge middleware that copies claims from the net/http request context into Gin’s context:

      engine.Use(func(c *gin.Context) {
          if claims := custom_jwt.GetClaimsFromContext(c.Request); claims != nil {
              c.Set("jwt_claims", claims) // jwt.MapClaims
          }
          c.Next()
      })

- Inside Gin handlers you can now retrieve claims using c.Get("jwt_claims") similar to:

      var claims any
      if value, exists := c.Get("jwt_claims"); exists {
          // value is jwt.MapClaims; adapt cast if your project uses a custom struct
          claims = value
      } else {
          claims = map[string]any{}
      }
- pkg/router mounts a migration-only mirror at /migration/user and adds two middlewares:
  - JWT verification: custom_jwt.NewJWTMiddleware(VerifyConfig{Secret: ...})
  - Authorization check: reads claims and verifies attrs.super_admin == true
- The example issues a token using custom_jwt.Config{Custom: map[string]any{"attrs": {"super_admin": true}}}.Issue().
- The program first requests without token (401) and then with token (200), and prints helpful curl commands.
