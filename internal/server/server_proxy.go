package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)


// handleLogReportProxy proxies /logreport/ requests to the LOGReport server on localhost:8642.
// This allows the remote VM (which can only reach port 80) to access the LOGReport web UI.
func (s *Server) handleLogReportProxy(w http.ResponseWriter, r *http.Request) {
	target, _ := url.Parse("http://localhost:8642")
	proxy := httputil.NewSingleHostReverseProxy(target)
	// Strip the /logreport prefix
	r.URL.Path = strings.TrimPrefix(r.URL.Path, "/logreport")
	if r.URL.Path == "" {
		r.URL.Path = "/"
	}
	proxy.ServeHTTP(w, r)
}

