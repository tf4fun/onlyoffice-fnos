package server

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// setupProxy configures the reverse proxy for Document Server
func (s *Server) setupProxy() {
	if s.settings == nil || s.settings.DocumentServerURL == "" {
		log.Println("Warning: Document Server URL not configured, proxy disabled")
		return
	}

	targetURL, err := url.Parse(s.settings.DocumentServerURL)
	if err != nil {
		log.Printf("Warning: invalid Document Server URL: %v", err)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Customize the Director to set X-Forwarded headers for OnlyOffice virtual path
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Strip /docserver prefix
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/docserver")
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}

		// Determine scheme
		scheme := "http"
		if req.TLS != nil {
			scheme = "https"
		}
		if fwdProto := req.Header.Get("X-Forwarded-Proto"); fwdProto != "" {
			scheme = fwdProto
		}

		// Determine host
		host := req.Host
		if fwdHost := req.Header.Get("X-Forwarded-Host"); fwdHost != "" {
			host = fwdHost
		}

		// Critical: Tell OnlyOffice its virtual path
		req.Header.Set("X-Forwarded-Host", host+"/docserver")
		req.Header.Set("X-Forwarded-Proto", scheme)
	}

	// Handle proxy errors
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy error: %v", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	s.router.Handle("/docserver/*", proxy)
	s.router.Handle("/docserver", proxy)
}
