package router

import (
	"net/http"
	"strings"
	"sync/atomic"
)

// Middleware is a function that wraps an http.Handler and returns another handler.
// It can be used to add cross-cutting concerns (e.g., authentication) to routes mounted on the Router.
// Since: 0.2.0
type Middleware func(http.Handler) http.Handler

// Options configures the migration router behavior.
// BasePath is the URL prefix under which migration-only endpoints are exposed, e.g. "/migration".
// If empty, it defaults to "/migration".
//
// This router is intentionally minimal and framework-agnostic. It lets you expose the same
// business APIs under a migration-only prefix so you can open them temporarily during migrations
// and then close them.
//
// Example:
//
//	appMux := http.NewServeMux()
//	appMux.HandleFunc("/user", userHandler)
//
//	// Create a migration router and mount the app handler under /migration/user
//	mr := router.New(Options{BasePath: "/migration"})
//	mr.MountHandler("/user", http.HandlerFunc(userHandler))
//	// Optionally disable after done: mr.Close()
//
//	// Attach both to your server
//	root := http.NewServeMux()
//	root.Handle("/", appMux)
//	root.Handle("/", mr) // mr only handles requests under BasePath
//
// Note: The first matching handler will process the request according to how you attach it to root mux.
// Ensure your mounting order doesn't shadow the migration prefix.
//
// Since: 0.2.0
// Experimental: API surface may evolve.
//
//nolint:revive // config carrier
type Options struct {
	BasePath string
}

// Router exposes handlers under a dedicated migration prefix and can be opened/closed.
// It implements http.Handler. When closed, all requests under BasePath return 404.
type Router struct {
	base string
	mux  *http.ServeMux
	open atomic.Bool
	mws  []Middleware
}

// New creates a new migration Router with the provided options.
func New(opt Options) *Router {
	bp := strings.TrimSpace(opt.BasePath)
	if bp == "" {
		bp = "/migration"
	}
	// ensure it starts with '/'
	if !strings.HasPrefix(bp, "/") {
		bp = "/" + bp
	}
	// ensure no trailing slash (except root)
	if len(bp) > 1 && strings.HasSuffix(bp, "/") {
		bp = strings.TrimSuffix(bp, "/")
	}
	r := &Router{base: bp, mux: http.NewServeMux()}
	r.open.Store(true)
	return r
}

// BasePath returns the configured prefix.
func (r *Router) BasePath() string { return r.base }

// Open enables serving of migration endpoints.
func (r *Router) Open() { r.open.Store(true) }

// Close disables serving; requests under BasePath will get 404.
func (r *Router) Close() { r.open.Store(false) }

// MountHandler mounts a handler under the migration base path with the given sub-path.
// For example, subPath "/user" will be served at BasePath+"/user".
func (r *Router) MountHandler(subPath string, h http.Handler) {
	sp := sanitizePath(subPath)
	final := h
	// apply middlewares in registration order
	for i := len(r.mws) - 1; i >= 0; i-- {
		final = r.mws[i](final)
	}
	r.mux.Handle(r.base+sp, final)
}

// HandleFunc adds a handler function for method-agnostic routes under the migration prefix.
func (r *Router) HandleFunc(subPath string, fn http.HandlerFunc) { r.MountHandler(subPath, fn) }

// Handle registers an http.Handler under an explicit full path relative to BasePath.
func (r *Router) Handle(subPath string, h http.Handler) { r.MountHandler(subPath, h) }

// Use registers one or more middlewares that will wrap handlers mounted after this call.
// Middlewares are applied in the order they were added: Use(A,B) wraps as A(B(handler)).
func (r *Router) Use(mw ...Middleware) {
	if len(mw) == 0 {
		return
	}
	r.mws = append(r.mws, mw...)
}

// ServeHTTP implements http.Handler, serving only requests whose URL path starts with BasePath.
// Other paths are ignored so that you can mount Router at "/" alongside your normal mux.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if !strings.HasPrefix(req.URL.Path, r.base) {
		// Not our prefix; let other handlers handle it.
		return
	}
	if !r.open.Load() {
		http.NotFound(w, req)
		return
	}
	// Delegate to inner mux. We keep the path intact since we registered full BasePath+subPath.
	r.mux.ServeHTTP(w, req)
}

func sanitizePath(p string) string {
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// collapse trailing slash except root
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}
