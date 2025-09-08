package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/loykin/apimigrate/pkg/router"
)

// This example shows how to expose an existing business API under a separate
// migration-only prefix (e.g., /migration/user) so you can open it temporarily
// for data migrations and then close it again.
func main() {
	// Business handler used by your normal application under /user
	userHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"path":"` + r.URL.Path + `"}`))
	})

	// Normal application mux
	appMux := http.NewServeMux()
	appMux.Handle("/user", userHandler)

	// Migration router mounted under /migration
	mr := router.New(router.Options{BasePath: "/migration"})
	// Mount the same handler so it is reachable via /migration/user
	mr.MountHandler("/user", userHandler)

	// Root mux combines both
	root := http.NewServeMux()
	root.Handle("/", appMux)
	root.Handle("/", mr) // mr will only handle requests with prefix /migration

	// For example purposes, we use httptest server so it runs anywhere
	srv := httptest.NewServer(root)
	defer srv.Close()

	log.Printf("server started: %s", srv.URL)
	fmt.Println("Normal API:      ", srv.URL+"/user")
	fmt.Println("Migration mirror:", srv.URL+"/migration/user")

	// Demonstrate open/close behavior
	resp1, _ := http.Get(srv.URL + "/migration/user")
	fmt.Println("GET /migration/user status before close:", resp1.StatusCode)
	_ = resp1.Body.Close()

	// Close migration routes to stop serving them
	mr.Close()

	resp2, _ := http.Get(srv.URL + "/migration/user")
	fmt.Println("GET /migration/user status after close: ", resp2.StatusCode)
	_ = resp2.Body.Close()

	// Re-open again (optional)
	mr.Open()
	resp3, _ := http.Get(srv.URL + "/migration/user")
	fmt.Println("GET /migration/user status after reopen:", resp3.StatusCode)
	_ = resp3.Body.Close()

	// Keep the example process alive briefly so users can curl manually
	fmt.Println("Try: curl", srv.URL+"/migration/user")
	fmt.Println("(it will be available for ~10 seconds, then program exits)")
	time.Sleep(10 * time.Second)
}
