package server

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// createDocServerProxy creates a reverse proxy handler for Document Server requests.
// It proxies requests from /doc-svr/* to the configured Document Server URL,
// stripping the /doc-svr prefix before forwarding.
func (s *Server) createDocServerProxy() http.Handler {
	// Parse the Document Server URL
	targetURL, err := url.Parse(s.settings.DocumentServerURL)
	if err != nil {
		log.Printf("Error parsing Document Server URL: %v", err)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Document Server URL not configured", http.StatusInternalServerError)
		})
	}

	// Create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Customize the Director to handle path rewriting and header processing
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		// Store original request info before calling original director
		originalHost := req.Host
		originalProto := getRequestProto(req)
		originalRemoteAddr := getClientIP(req)
		// Capture existing X-Forwarded-For for chained proxy scenarios
		existingXForwardedFor := req.Header.Get("X-Forwarded-For")

		// Call the original director first
		originalDirector(req)

		// Strip the /doc-svr prefix from the path
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/doc-svr")
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}

		// Preserve the raw path for proper encoding
		req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, "/doc-svr")

		// Set the Host header to the target host
		req.Host = targetURL.Host

		// Set X-Forwarded-Host and X-Forwarded-Proto
		if originalHost != "" {
			req.Header.Set("X-Forwarded-Host", originalHost)
		}
		if originalProto != "" {
			req.Header.Set("X-Forwarded-Proto", originalProto)
		}

		// For X-Forwarded-For, we need special handling because ReverseProxy
		// appends client IP to X-Forwarded-For AFTER Director returns.
		// According to the documentation: "if the header exists in the Request.Header map
		// but has a nil value, the X-Forwarded-For header is not modified."
		// So we set it to nil first, then ReverseProxy won't modify it.
		// We store our desired value in a custom header and restore it in the transport.
		if originalRemoteAddr != "" {
			var xffValue string
			if existingXForwardedFor != "" {
				xffValue = existingXForwardedFor + ", " + originalRemoteAddr
			} else {
				xffValue = originalRemoteAddr
			}
			// Store the value we want in a temporary header
			req.Header.Set("X-Kiro-Forwarded-For", xffValue)
			// Set X-Forwarded-For to nil to prevent ReverseProxy from modifying it
			req.Header["X-Forwarded-For"] = nil
		}

		// Preserve WebSocket upgrade headers
		preserveWebSocketHeaders(req)
	}

	// Use a custom transport to restore the X-Forwarded-For header
	originalTransport := proxy.Transport
	if originalTransport == nil {
		originalTransport = http.DefaultTransport
	}
	proxy.Transport = &xffPreservingTransport{
		base: originalTransport,
	}

	return proxy
}

// xffPreservingTransport is a custom transport that restores the X-Forwarded-For header
// from our temporary header before sending the request.
type xffPreservingTransport struct {
	base http.RoundTripper
}

func (t *xffPreservingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Restore X-Forwarded-For from our temporary header
	if xff := req.Header.Get("X-Kiro-Forwarded-For"); xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
		req.Header.Del("X-Kiro-Forwarded-For")
	}
	return t.base.RoundTrip(req)
}

// preserveWebSocketHeaders ensures WebSocket upgrade headers are preserved
// during proxying. This is essential for WebSocket connections to work properly.
func preserveWebSocketHeaders(req *http.Request) {
	// The Upgrade and Connection headers are needed for WebSocket handshake
	// httputil.ReverseProxy by default removes hop-by-hop headers including Connection
	// We need to ensure these are preserved for WebSocket upgrades

	// Check if this is a WebSocket upgrade request
	if isWebSocketUpgrade(req) {
		// Ensure Upgrade header is preserved (usually "websocket")
		if upgrade := req.Header.Get("Upgrade"); upgrade != "" {
			req.Header.Set("Upgrade", upgrade)
		}

		// Ensure Connection header includes "Upgrade"
		// The Connection header should be "Upgrade" for WebSocket handshake
		if connection := req.Header.Get("Connection"); connection != "" {
			req.Header.Set("Connection", connection)
		}

		// Preserve WebSocket-specific headers
		// Sec-WebSocket-Key is required for the handshake
		if wsKey := req.Header.Get("Sec-WebSocket-Key"); wsKey != "" {
			req.Header.Set("Sec-WebSocket-Key", wsKey)
		}

		// Sec-WebSocket-Version indicates the WebSocket protocol version
		if wsVersion := req.Header.Get("Sec-WebSocket-Version"); wsVersion != "" {
			req.Header.Set("Sec-WebSocket-Version", wsVersion)
		}

		// Sec-WebSocket-Protocol for subprotocol negotiation
		if wsProtocol := req.Header.Get("Sec-WebSocket-Protocol"); wsProtocol != "" {
			req.Header.Set("Sec-WebSocket-Protocol", wsProtocol)
		}

		// Sec-WebSocket-Extensions for extension negotiation
		if wsExtensions := req.Header.Get("Sec-WebSocket-Extensions"); wsExtensions != "" {
			req.Header.Set("Sec-WebSocket-Extensions", wsExtensions)
		}
	}
}

// isWebSocketUpgrade checks if the request is a WebSocket upgrade request.
func isWebSocketUpgrade(req *http.Request) bool {
	// A WebSocket upgrade request has:
	// - Upgrade header containing "websocket" (case-insensitive)
	// - Connection header containing "Upgrade" (case-insensitive)
	upgrade := strings.ToLower(req.Header.Get("Upgrade"))
	connection := strings.ToLower(req.Header.Get("Connection"))

	return strings.Contains(upgrade, "websocket") &&
		strings.Contains(connection, "upgrade")
}

// getRequestProto determines the protocol (http/https) of the original request.
// It checks X-Forwarded-Proto first (in case of chained proxies), then TLS status.
func getRequestProto(req *http.Request) string {
	// Check if there's already an X-Forwarded-Proto header (chained proxy scenario)
	if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}

	// Check TLS status
	if req.TLS != nil {
		return "https"
	}

	// Default to http
	return "http"
}

// getClientIP extracts the client IP address from the request.
// It uses the original RemoteAddr stored in context (before RealIP middleware modified it).
func getClientIP(req *http.Request) string {
	// Get the original RemoteAddr from context (set by CaptureOriginalRemoteAddr middleware)
	remoteAddr := GetOriginalRemoteAddr(req)
	if remoteAddr == "" {
		return ""
	}

	// Handle IPv6 addresses in brackets
	if strings.HasPrefix(remoteAddr, "[") {
		// IPv6 format: [::1]:port
		if idx := strings.LastIndex(remoteAddr, "]:"); idx != -1 {
			return remoteAddr[1:idx]
		}
		// IPv6 without port: [::1]
		return strings.Trim(remoteAddr, "[]")
	}

	// Handle IPv4 addresses
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}

	return remoteAddr
}

// handleDocServerProxy handles requests to /doc-svr/* and proxies them to Document Server
func (s *Server) handleDocServerProxy(w http.ResponseWriter, r *http.Request) {
	// Check if Document Server URL is configured
	if s.settings == nil || s.settings.DocumentServerURL == "" {
		http.Error(w, "Document Server URL not configured", http.StatusInternalServerError)
		return
	}

	proxy := s.createDocServerProxy()
	proxy.ServeHTTP(w, r)
}
