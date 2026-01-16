package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"onlyoffice-fnos/internal/config"
)

func TestDocServerProxyXForwardedHeaders(t *testing.T) {
	// Create a mock Document Server that captures headers
	var capturedHeaders http.Header
	mockDocServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockDocServer.Close()

	// Create server with mock Document Server URL
	srv := New(&Config{
		Settings: &config.Settings{
			DocumentServerURL: mockDocServer.URL,
		},
	})

	tests := []struct {
		name                  string
		requestHost           string
		requestProto          string // Set via X-Forwarded-Proto on incoming request
		remoteAddr            string
		existingXForwardedFor string
		expectedHost          string
		expectedProto         string
		expectedForContains   string
	}{
		{
			name:                "basic request with host",
			requestHost:         "example.com:5666",
			remoteAddr:          "192.168.1.100:12345",
			expectedHost:        "example.com:5666",
			expectedProto:       "http",
			expectedForContains: "192.168.1.100",
		},
		{
			name:                "request with existing X-Forwarded-Proto",
			requestHost:         "example.com",
			requestProto:        "https",
			remoteAddr:          "10.0.0.1:8080",
			expectedHost:        "example.com",
			expectedProto:       "https",
			expectedForContains: "10.0.0.1",
		},
		{
			name:                  "chained proxy with existing X-Forwarded-For",
			requestHost:           "myhost.local",
			remoteAddr:            "172.16.0.1:9999",
			existingXForwardedFor: "203.0.113.50",
			expectedHost:          "myhost.local",
			expectedProto:         "http",
			expectedForContains:   "203.0.113.50, 172.16.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/doc-svr/test", nil)
			req.Host = tt.requestHost
			req.RemoteAddr = tt.remoteAddr

			if tt.requestProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.requestProto)
			}
			if tt.existingXForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.existingXForwardedFor)
			}

			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
			}

			// Check X-Forwarded-Host
			if got := capturedHeaders.Get("X-Forwarded-Host"); got != tt.expectedHost {
				t.Errorf("X-Forwarded-Host: expected %q, got %q", tt.expectedHost, got)
			}

			// Check X-Forwarded-Proto
			if got := capturedHeaders.Get("X-Forwarded-Proto"); got != tt.expectedProto {
				t.Errorf("X-Forwarded-Proto: expected %q, got %q", tt.expectedProto, got)
			}

			// Check X-Forwarded-For contains expected value
			if got := capturedHeaders.Get("X-Forwarded-For"); got != tt.expectedForContains {
				t.Errorf("X-Forwarded-For: expected %q, got %q", tt.expectedForContains, got)
			}
		})
	}
}

func TestDocServerProxyWebSocketUpgrade(t *testing.T) {
	// Create a mock Document Server that captures headers
	var capturedHeaders http.Header
	mockDocServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))
	defer mockDocServer.Close()

	// Create server with mock Document Server URL
	srv := New(&Config{
		Settings: &config.Settings{
			DocumentServerURL: mockDocServer.URL,
		},
	})

	tests := []struct {
		name            string
		headers         map[string]string
		expectUpgrade   string
		expectConnection string
		expectWSKey     string
		expectWSVersion string
	}{
		{
			name: "WebSocket upgrade request",
			headers: map[string]string{
				"Upgrade":               "websocket",
				"Connection":            "Upgrade",
				"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
				"Sec-WebSocket-Version": "13",
			},
			expectUpgrade:    "websocket",
			expectConnection: "Upgrade",
			expectWSKey:      "dGhlIHNhbXBsZSBub25jZQ==",
			expectWSVersion:  "13",
		},
		{
			name: "WebSocket with protocol and extensions",
			headers: map[string]string{
				"Upgrade":                  "websocket",
				"Connection":               "Upgrade",
				"Sec-WebSocket-Key":        "x3JJHMbDL1EzLkh9GBhXDw==",
				"Sec-WebSocket-Version":    "13",
				"Sec-WebSocket-Protocol":   "chat, superchat",
				"Sec-WebSocket-Extensions": "permessage-deflate",
			},
			expectUpgrade:    "websocket",
			expectConnection: "Upgrade",
			expectWSKey:      "x3JJHMbDL1EzLkh9GBhXDw==",
			expectWSVersion:  "13",
		},
		{
			name: "case insensitive WebSocket headers",
			headers: map[string]string{
				"Upgrade":               "WebSocket",
				"Connection":            "upgrade",
				"Sec-WebSocket-Key":     "testkey123==",
				"Sec-WebSocket-Version": "13",
			},
			expectUpgrade:    "WebSocket",
			expectConnection: "Upgrade", // Go's http package normalizes "upgrade" to "Upgrade"
			expectWSKey:      "testkey123==",
			expectWSVersion:  "13",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/doc-svr/socket", nil)
			req.RemoteAddr = "127.0.0.1:12345"

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			// Check Upgrade header is preserved
			if got := capturedHeaders.Get("Upgrade"); got != tt.expectUpgrade {
				t.Errorf("Upgrade header: expected %q, got %q", tt.expectUpgrade, got)
			}

			// Check Connection header is preserved
			if got := capturedHeaders.Get("Connection"); got != tt.expectConnection {
				t.Errorf("Connection header: expected %q, got %q", tt.expectConnection, got)
			}

			// Check Sec-WebSocket-Key is preserved
			if got := capturedHeaders.Get("Sec-WebSocket-Key"); got != tt.expectWSKey {
				t.Errorf("Sec-WebSocket-Key header: expected %q, got %q", tt.expectWSKey, got)
			}

			// Check Sec-WebSocket-Version is preserved
			if got := capturedHeaders.Get("Sec-WebSocket-Version"); got != tt.expectWSVersion {
				t.Errorf("Sec-WebSocket-Version header: expected %q, got %q", tt.expectWSVersion, got)
			}

			// Check optional headers if they were set
			if proto, ok := tt.headers["Sec-WebSocket-Protocol"]; ok {
				if got := capturedHeaders.Get("Sec-WebSocket-Protocol"); got != proto {
					t.Errorf("Sec-WebSocket-Protocol header: expected %q, got %q", proto, got)
				}
			}

			if ext, ok := tt.headers["Sec-WebSocket-Extensions"]; ok {
				if got := capturedHeaders.Get("Sec-WebSocket-Extensions"); got != ext {
					t.Errorf("Sec-WebSocket-Extensions header: expected %q, got %q", ext, got)
				}
			}
		})
	}
}

func TestIsWebSocketUpgrade(t *testing.T) {
	tests := []struct {
		name       string
		upgrade    string
		connection string
		expected   bool
	}{
		{
			name:       "valid WebSocket upgrade",
			upgrade:    "websocket",
			connection: "Upgrade",
			expected:   true,
		},
		{
			name:       "case insensitive",
			upgrade:    "WebSocket",
			connection: "upgrade",
			expected:   true,
		},
		{
			name:       "connection with multiple values",
			upgrade:    "websocket",
			connection: "keep-alive, Upgrade",
			expected:   true,
		},
		{
			name:       "missing upgrade header",
			upgrade:    "",
			connection: "Upgrade",
			expected:   false,
		},
		{
			name:       "missing connection header",
			upgrade:    "websocket",
			connection: "",
			expected:   false,
		},
		{
			name:       "wrong upgrade value",
			upgrade:    "h2c",
			connection: "Upgrade",
			expected:   false,
		},
		{
			name:       "connection without upgrade",
			upgrade:    "websocket",
			connection: "keep-alive",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.upgrade != "" {
				req.Header.Set("Upgrade", tt.upgrade)
			}
			if tt.connection != "" {
				req.Header.Set("Connection", tt.connection)
			}

			result := isWebSocketUpgrade(req)
			if result != tt.expected {
				t.Errorf("isWebSocketUpgrade() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		expected   string
	}{
		{
			name:       "IPv4 with port",
			remoteAddr: "192.168.1.100:12345",
			expected:   "192.168.1.100",
		},
		{
			name:       "IPv6 with port",
			remoteAddr: "[::1]:12345",
			expected:   "::1",
		},
		{
			name:       "IPv6 full address with port",
			remoteAddr: "[2001:db8::1]:8080",
			expected:   "2001:db8::1",
		},
		{
			name:       "IPv4 without port",
			remoteAddr: "10.0.0.1",
			expected:   "10.0.0.1",
		},
		{
			name:       "empty remote addr",
			remoteAddr: "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			result := getClientIP(req)
			if result != tt.expected {
				t.Errorf("getClientIP() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestGetRequestProto(t *testing.T) {
	tests := []struct {
		name           string
		xForwardedProto string
		useTLS         bool
		expected       string
	}{
		{
			name:           "X-Forwarded-Proto https",
			xForwardedProto: "https",
			useTLS:         false,
			expected:       "https",
		},
		{
			name:           "X-Forwarded-Proto http",
			xForwardedProto: "http",
			useTLS:         true, // X-Forwarded-Proto takes precedence
			expected:       "http",
		},
		{
			name:           "no header with TLS",
			xForwardedProto: "",
			useTLS:         true,
			expected:       "https",
		},
		{
			name:           "no header without TLS",
			xForwardedProto: "",
			useTLS:         false,
			expected:       "http",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.xForwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.xForwardedProto)
			}
			if tt.useTLS {
				req.TLS = &tls.ConnectionState{} // Non-nil TLS indicates HTTPS
			}

			result := getRequestProto(req)
			if result != tt.expected {
				t.Errorf("getRequestProto() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestDocServerProxyPathRewrite(t *testing.T) {
	// Create a mock Document Server
	mockDocServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back the received path
		w.Header().Set("X-Received-Path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockDocServer.Close()

	// Create server with mock Document Server URL
	srv := New(&Config{
		Settings: &config.Settings{
			DocumentServerURL: mockDocServer.URL,
		},
	})

	tests := []struct {
		name         string
		requestPath  string
		expectedPath string
	}{
		{
			name:         "simple path",
			requestPath:  "/doc-svr/web-apps/apps/api/documents/api.js",
			expectedPath: "/web-apps/apps/api/documents/api.js",
		},
		{
			name:         "root path with trailing slash",
			requestPath:  "/doc-svr/",
			expectedPath: "/",
		},
		{
			name:         "nested path",
			requestPath:  "/doc-svr/cache/files/data.json",
			expectedPath: "/cache/files/data.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
			}

			receivedPath := rec.Header().Get("X-Received-Path")
			if receivedPath != tt.expectedPath {
				t.Errorf("Expected path %q, got %q", tt.expectedPath, receivedPath)
			}
		})
	}
}

func TestDocServerProxyNotConfigured(t *testing.T) {
	// Create server without Document Server URL
	srv := New(&Config{
		Settings: &config.Settings{
			DocumentServerURL: "",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/doc-svr/test", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestDocServerProxyNilSettings(t *testing.T) {
	// Create server with nil settings
	srv := New(&Config{
		Settings: nil,
	})

	req := httptest.NewRequest(http.MethodGet, "/doc-svr/test", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}
