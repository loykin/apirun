package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loykin/apimigrate"
	"github.com/loykin/apimigrate/pkg/env"
)

// This example demonstrates how to:
// - Use Gin for your normal app routes (e.g., GET /user)
// - Open a custom migration router under /migration using Gin routes
// - Perform migrations via HTTP endpoints mounted on the migration router
// - Mirror an existing business handler under /migration/user for migration-only access
func main() {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	// A business handler used by the app under /user
	engine.GET("/user", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "path": c.Request.URL.Path})
	})

	// Migration sub-router (group) under /migration
	mg := engine.Group("/migration")

	// Mirror the same business handler under /migration/user (can be opened/closed by removing/adding this route in real apps)
	mg.GET("/user", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "path": c.Request.URL.Path})
	})

	// In this example, we will run migrations against the sample files in examples/embedded/migration.
	// We'll store migration state in a temporary sqlite file so the repository remains unchanged.
	tmpDir, _ := os.MkdirTemp("", "apimigrate-gin-*")
	defer func() { _ = os.RemoveAll(tmpDir) }()
	storePath := filepath.Join(tmpDir, "apimigrate.db")

	migrateDir := "./examples/embedded/migration"
	baseEnv := env.New()

	// Helper to build a Migrator with our shared options
	newMigrator := func() apimigrate.Migrator {
		cfg := apimigrate.StoreConfig{}
		cfg.DriverConfig = &apimigrate.SqliteConfig{Path: storePath}
		return apimigrate.Migrator{Dir: migrateDir, Env: baseEnv, StoreConfig: &cfg}
	}

	// POST /migration/up?to=N -> run migrations up to N (0 = all)
	mg.POST("/up", func(c *gin.Context) {
		to := atoiDefault(c.Query("to"), 0)
		m := newMigrator()
		res, err := m.MigrateUp(c.Request.Context(), to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"applied": len(res)})
	})

	// POST /migration/down?to=N -> rollback down to N
	mg.POST("/down", func(c *gin.Context) {
		to := atoiDefault(c.Query("to"), 0)
		m := newMigrator()
		res, err := m.MigrateDown(c.Request.Context(), to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"rolled_back": len(res)})
	})

	// GET /migration/status -> return latest applied version
	mg.GET("/status", func(c *gin.Context) {
		sc := apimigrate.StoreConfig{}
		sc.DriverConfig = &apimigrate.SqliteConfig{Path: storePath}
		st, err := apimigrate.OpenStoreFromOptions(migrateDir, &sc)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer func() { _ = (*st).Close() }()
		list, err := (*st).ListApplied()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		latest := 0
		if len(list) > 0 {
			latest = list[len(list)-1]
		}
		c.JSON(http.StatusOK, gin.H{"applied": list, "latest": latest})
	})

	// For example purposes, run this Gin engine on an httptest server so it works anywhere
	srv := httptest.NewServer(engine.Handler())
	defer srv.Close()

	log.Printf("gin server started: %s", srv.URL)
	fmt.Println("Normal API:       ", srv.URL+"/user")
	fmt.Println("Migration mirror:  ", srv.URL+"/migration/user")
	fmt.Println("Migration up:      ", "POST "+srv.URL+"/migration/up?to=0")
	fmt.Println("Migration status:  ", srv.URL+"/migration/status")

	// Demonstrate basic flow (call the endpoints programmatically)
	client := &http.Client{Timeout: 10 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1) Check mirror
	resp1, _ := client.Get(srv.URL + "/migration/user")
	fmt.Println("GET /migration/user:", resp1.StatusCode)
	_ = resp1.Body.Close()

	// 2) Run up to all
	reqUp, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/migration/up?to=0", nil)
	respUp, _ := client.Do(reqUp)
	fmt.Println("POST /migration/up?to=0:", respUp.StatusCode)
	_ = respUp.Body.Close()

	// 3) Query status
	respSt, _ := client.Get(srv.URL + "/migration/status")
	fmt.Println("GET /migration/status:", respSt.StatusCode)
	_ = respSt.Body.Close()

	fmt.Println("Try these commands in a separate shell while the program runs:")
	fmt.Println(" curl -i ", srv.URL+"/migration/user")
	fmt.Println(" curl -i -X POST ", srv.URL+"/migration/up?to=0")
	fmt.Println(" curl -i ", srv.URL+"/migration/status")

	// keep alive briefly
	time.Sleep(10 * time.Second)
}

func atoiDefault(s string, def int) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return i
}
