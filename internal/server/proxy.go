package server

import (
	"log"
	"net"
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

		// Determine host first
		host := req.Host
		if fwdHost := req.Header.Get("X-Forwarded-Host"); fwdHost != "" {
			host = fwdHost
		}

		// Determine scheme - check multiple headers that proxies might set
		scheme := "http"
		if req.TLS != nil {
			scheme = "https"
		}
		if fwdProto := req.Header.Get("X-Forwarded-Proto"); fwdProto != "" {
			scheme = fwdProto
		}
		if fwdScheme := req.Header.Get("X-Forwarded-Scheme"); fwdScheme != "" {
			scheme = fwdScheme
		}
		if xScheme := req.Header.Get("X-Scheme"); xScheme != "" {
			scheme = xScheme
		}

		// If host looks like an external domain (not IP, not localhost), assume HTTPS
		// This handles cases where upstream proxy doesn't set X-Forwarded-Proto
		if scheme == "http" && !isInternalHost(host) {
			scheme = "https"
		}

		// Log for debugging proxy issues
		log.Printf("Proxy: path=%s, scheme=%s, host=%s, X-Forwarded-Proto=%s",
			req.URL.Path, scheme, host, req.Header.Get("X-Forwarded-Proto"))

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


// isInternalHost checks if the host looks like an internal/local address
func isInternalHost(host string) bool {
	// Remove port if present
	h := host
	if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
		h = host[:colonIdx]
	}

	// Check for localhost
	if h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}

	// Check if it's an IP address
	ip := net.ParseIP(h)
	if ip != nil {
		// Check for private IP ranges
		return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
	}

	// It's a domain name - assume external
	return false
}
