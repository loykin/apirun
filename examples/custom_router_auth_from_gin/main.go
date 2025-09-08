package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loykin/apimigrate/internal/auth/custom_jwt"
	"github.com/loykin/apimigrate/pkg/router"
)

// This example shows how to:
// - Define a normal Gin route (GET /user)
// - Expose a migration-only mirror at /migration/user via pkg/router
// - Protect the migration routes with JWT middleware
// - Issue a JWT with custom payload {"attrs": {"super_admin": true}} and authorize based on it
func main() {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	// Bridge middleware: copy claims from net/http request context into Gin context under key "jwt_claims"
	engine.Use(func(c *gin.Context) {
		if claims := custom_jwt.GetClaimsFromContext(c.Request); claims != nil {
			c.Set("jwt_claims", claims)
		}
		c.Next()
	})

	// Normal app route: demonstrate retrieving JWT claims using c.Get("jwt_claims")
	engine.GET("/user", func(c *gin.Context) {
		var claims any
		if value, exists := c.Get("jwt_claims"); exists {
			// In this repo we store jwt.MapClaims; adapt cast if you use your own types
			claims = value
		} else {
			claims = map[string]any{}
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "path": c.Request.URL.Path, "claims_present": claims != nil})
	})

	// Create migration router under /migration
	mr := router.New(router.Options{BasePath: "/migration"})

	// Shared secret for HS256
	secret := []byte("dev-secret-change-me")

	// JWT verification middleware
	jwtMw := custom_jwt.NewJWTMiddleware(custom_jwt.VerifyConfig{
		Secret:     secret,
		RequireJTI: false,
		ClockSkew:  2 * time.Second,
	})

	// Authorization middleware: require attrs.super_admin == true
	authzMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := custom_jwt.GetClaimsFromContext(r)
			if claims == nil {
				http.Error(w, "no claims", http.StatusUnauthorized)
				return
			}
			attrs, _ := claims["attrs"].(map[string]interface{})
			if okv, _ := attrs["super_admin"].(bool); !okv {
				http.Error(w, "forbidden: super_admin required", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	// Register middlewares on the migration router
	mr.Use(jwtMw, authzMw)

	// Mount migration-only mirror that forwards to Gin's /user
	mr.MountHandler("/user", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/user"
		engine.ServeHTTP(w, r2)
	}))

	// Combine into root mux
	root := http.NewServeMux()
	root.Handle("/", engine)
	root.Handle("/", mr)

	srv := httptest.NewServer(root)
	defer srv.Close()

	log.Printf("server: %s", srv.URL)
	fmt.Println("Normal API:          ", srv.URL+"/user")
	fmt.Println("Migration mirror:     ", srv.URL+"/migration/user")

	// Issue a JWT with custom payload attrs.super_admin = true
	tok, err := custom_jwt.Config{
		Secret:     string(secret),
		TTLSeconds: 300,
		Custom: map[string]any{
			"attrs": map[string]any{"super_admin": true},
		},
	}.Issue()
	if err != nil {
		log.Fatalf("failed to issue token: %v", err)
	}
	bearer := "Bearer " + tok

	client := &http.Client{Timeout: 5 * time.Second}

	// 1) Without token -> 401
	resp1, _ := client.Get(srv.URL + "/migration/user")
	fmt.Println("GET /migration/user without token:", resp1.StatusCode)
	_ = resp1.Body.Close()

	// 2) With token -> 200
	req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/migration/user", nil)
	req2.Header.Set("Authorization", bearer)
	resp2, _ := client.Do(req2)
	fmt.Println("GET /migration/user with token:   ", resp2.StatusCode)
	_ = resp2.Body.Close()

	// Print helper curl commands
	fmt.Println("Try in another shell while it runs (~10s):")
	fmt.Println(" curl -i ", srv.URL+"/migration/user")
	fmt.Println(" TOKEN=", bearer)
	fmt.Println(" curl -i -H 'Authorization: "+bearer+"' ", srv.URL+"/migration/user")

	// Keep alive briefly
	time.Sleep(10 * time.Second)
}
