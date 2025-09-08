package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loykin/apimigrate/pkg/router"
)

// Example: Register an existing Gin handler into apimigrate's custom migration router (pkg/router)
// and serve it under a migration-only prefix separate from the normal service routes.
func main() {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	// Normal business endpoint using Gin at /user
	engine.GET("/user", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "path": c.Request.URL.Path})
	})

	// Create a migration router mounted under /migration
	mr := router.New(router.Options{BasePath: "/migration"})

	// Mount a migration-only mirror for the Gin /user route at /migration/user.
	// We rewrite the request URL path so Gin can match its original route definitions.
	mr.MountHandler("/user", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/user"
		engine.ServeHTTP(w, r2)
	}))

	// Compose root mux with both normal app and migration router
	root := http.NewServeMux()
	root.Handle("/", engine) // normal routes (e.g., /user)
	root.Handle("/", mr)     // migration routes (e.g., /migration/user)

	// Start test server so the example is self-contained
	srv := httptest.NewServer(root)
	defer srv.Close()

	log.Printf("server started: %s", srv.URL)
	fmt.Println("Normal API:        ", srv.URL+"/user")
	fmt.Println("Migration mirror:   ", srv.URL+"/migration/user")

	// Quick demo flow
	if resp, err := http.Get(srv.URL + "/migration/user"); err == nil {
		fmt.Println("GET /migration/user:", resp.StatusCode)
		_ = resp.Body.Close()
	}

	fmt.Println("Try in another shell while it runs (~8s):")
	fmt.Println(" curl -i ", srv.URL+"/migration/user")

	time.Sleep(8 * time.Second)
}
