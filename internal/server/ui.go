package server

import (
	"io/fs"
	"log"
	"net/http"
	"strings"

	probe "github.com/falke-ai-circuit/probe" // root embed package (package name: assets, provides FS)
)

// isReservedPath returns true if the given URL path is handled by a
// non-UI route (API, WebSocket, health, downloads, OpenAPI spec).
func isReservedPath(path string) bool {
	return strings.HasPrefix(path, "/api/") ||
		path == "/ws" ||
		strings.HasPrefix(path, "/ws/") ||
		path == "/health" ||
		strings.HasPrefix(path, "/download/") ||
		strings.HasPrefix(path, "/logreport/") ||
		path == "/openapi.json"
}



// registerUI sets up the embedded frontend serving. Instead of registering
// a catch-all "/" handler on the mux (which would interfere with Go 1.22's
// method-based 405 handling), this wraps the server's http.Handler with a
// UI layer that serves static files and SPA fallback for non-API paths.
//
// Must be called after registerRoutes but before Start/StartTLS sets up
// the http.Server. The wrapper is stored on the server and applied in
// Start/StartTLS.
func (s *Server) registerUI() {
	// Check if frontend is available (index.html exists in embed).
	distFS, err := fs.Sub(probe.FS, "web/dist")
	if err != nil {
		log.Printf("[ui] WARNING: embedded web/dist not available: %v", err)
		return
	}
	if _, err := fs.ReadFile(distFS, "index.html"); err != nil {
		log.Printf("[ui] WARNING: embedded index.html not found (frontend not built?): %v", err)
		return
	}

	// Wrap the mux with the UI handler.
	s.uiWrapper = s.makeUIHandler(distFS)
}

// makeUIHandler creates an http.Handler that serves static files and SPA
// fallback for non-reserved paths, and delegates to the mux for reserved paths.
func (s *Server) makeUIHandler(distFS fs.FS) http.Handler {
	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		log.Printf("[ui] WARNING: cannot read index.html: %v", err)
		return s.mux
	}

	fileServer := http.FileServer(http.FS(distFS))

	log.Printf("[ui] embedded frontend registered at /")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reserved paths → delegate to mux (preserves 405s, API routes, etc.)
		if isReservedPath(r.URL.Path) {
			s.mux.ServeHTTP(w, r)
			return
		}

		// Non-reserved paths → try static file.
		filePath := strings.TrimPrefix(r.URL.Path, "/")
		if filePath == "" {
			filePath = "index.html"
		}

		if f, err := distFS.Open(filePath); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(indexHTML)
	})
}