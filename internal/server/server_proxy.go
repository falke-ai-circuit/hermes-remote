package server

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// ProxyEntry defines a reverse proxy: requests to PathPrefix are forwarded
// to TargetURL with the prefix stripped.
type ProxyEntry struct {
	PathPrefix string // e.g. "/logreport"
	TargetURL  string // e.g. "http://localhost:8642"
}

// SetProxies configures multiple reverse proxy routes. Each entry maps a
// path prefix to a target URL. The prefix is stripped before forwarding.
// When no proxies are configured, the default LOGReport proxy is added
// automatically for backward compatibility.
func (s *Server) SetProxies(proxies []ProxyEntry) {
	s.proxyMu.Lock()
	defer s.proxyMu.Unlock()
	s.proxies = make(map[string]*ProxyEntry)
	for _, p := range proxies {
		target, err := url.Parse(p.TargetURL)
		if err != nil {
			log.Printf("[server] invalid proxy target %s: %v", p.TargetURL, err)
			continue
		}
		s.proxies[p.PathPrefix] = &ProxyEntry{
			PathPrefix: p.PathPrefix,
			TargetURL:  p.TargetURL,
		}
		// Register the route dynamically
		if s.mux != nil {
			s.mux.HandleFunc(p.PathPrefix+"/", s.makeProxyHandler(p.PathPrefix, target))
		}
	}
}

// proxyEntryForPath returns the proxy entry for the given request path, or nil.
func (s *Server) proxyEntryForPath(path string) (*ProxyEntry, *url.URL) {
	s.proxyMu.RLock()
	defer s.proxyMu.RUnlock()
	for prefix, entry := range s.proxies {
		if strings.HasPrefix(path, prefix+"/") || path == prefix {
			target, _ := url.Parse(entry.TargetURL)
			return entry, target
		}
	}
	return nil, nil
}

// makeProxyHandler creates an http.Handler for a reverse proxy that strips
// the path prefix before forwarding to the target.
func (s *Server) makeProxyHandler(prefix string, target *url.URL) http.HandlerFunc {
	proxy := httputil.NewSingleHostReverseProxy(target)
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		proxy.ServeHTTP(w, r)
	}
}

// handleLogReportProxy is kept for backward compatibility. It proxies
// /logreport/ requests to localhost:8642. When the configurable proxy
// system is active, this handler is not used (the proxy is registered
// as a dynamic route instead).
func (s *Server) handleLogReportProxy(w http.ResponseWriter, r *http.Request) {
	// Check if a configurable proxy is registered for /logreport
	if entry, target := s.proxyEntryForPath(r.URL.Path); entry != nil && target != nil {
		handler := s.makeProxyHandler(entry.PathPrefix, target)
		handler(w, r)
		return
	}
	// Fallback: hardcoded LOGReport proxy (backward compat when no proxies configured)
	target, _ := url.Parse("http://localhost:8642")
	proxy := httputil.NewSingleHostReverseProxy(target)
	r.URL.Path = strings.TrimPrefix(r.URL.Path, "/logreport")
	if r.URL.Path == "" {
		r.URL.Path = "/"
	}
	proxy.ServeHTTP(w, r)
}