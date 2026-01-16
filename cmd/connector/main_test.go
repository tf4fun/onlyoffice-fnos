package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// TestExtractPathFromRequestURI tests the path extraction logic
// Requirements: 3.2, 3.3, 3.4, 3.5
func TestExtractPathFromRequestURI(t *testing.T) {
	tests := []struct {
		name          string
		requestURI    string
		expectedPath  string
		expectedQuery string
	}{
		// Requirement 3.3: Extract path after go-index.cgi
		{
			name:          "editor with query string",
			requestURI:    "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi/editor?path=/vol1/test.docx",
			expectedPath:  "/editor",
			expectedQuery: "path=/vol1/test.docx",
		},
		{
			name:          "doc-svr path",
			requestURI:    "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi/doc-svr/web-apps/apps/api/documents/api.js",
			expectedPath:  "/doc-svr/web-apps/apps/api/documents/api.js",
			expectedQuery: "",
		},
		// Requirement 3.4: Empty path defaults to "/"
		{
			name:          "empty path after marker",
			requestURI:    "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi",
			expectedPath:  "/",
			expectedQuery: "",
		},
		{
			name:          "trailing slash only",
			requestURI:    "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi/",
			expectedPath:  "/",
			expectedQuery: "",
		},
		// Requirement 3.5: Correctly parse and separate path and query
		{
			name:          "path with multiple query params",
			requestURI:    "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi/callback?path=/vol1/doc.docx&action=save",
			expectedPath:  "/callback",
			expectedQuery: "path=/vol1/doc.docx&action=save",
		},
		{
			name:          "query string only (no path)",
			requestURI:    "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi?foo=bar",
			expectedPath:  "/",
			expectedQuery: "foo=bar",
		},
		// Edge cases
		{
			name:          "marker not found",
			requestURI:    "/some/other/path",
			expectedPath:  "/",
			expectedQuery: "",
		},
		{
			name:          "empty REQUEST_URI",
			requestURI:    "",
			expectedPath:  "/",
			expectedQuery: "",
		},
		{
			name:          "nested path with query",
			requestURI:    "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi/api/v1/files?id=123&format=json",
			expectedPath:  "/api/v1/files",
			expectedQuery: "id=123&format=json",
		},
		{
			name:          "path with special characters",
			requestURI:    "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi/download?path=/vol1/文档.docx",
			expectedPath:  "/download",
			expectedQuery: "path=/vol1/文档.docx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the REQUEST_URI environment variable (Requirement 3.2)
			os.Setenv("REQUEST_URI", tt.requestURI)
			defer os.Unsetenv("REQUEST_URI")

			path, query := extractPathFromRequestURI()

			if path != tt.expectedPath {
				t.Errorf("extractPathFromRequestURI() path = %q, want %q", path, tt.expectedPath)
			}
			if query != tt.expectedQuery {
				t.Errorf("extractPathFromRequestURI() query = %q, want %q", query, tt.expectedQuery)
			}
		})
	}
}

// TestExtractPathFromRequestURI_NoEnvVar tests behavior when REQUEST_URI is not set
func TestExtractPathFromRequestURI_NoEnvVar(t *testing.T) {
	// Ensure REQUEST_URI is not set
	os.Unsetenv("REQUEST_URI")

	path, query := extractPathFromRequestURI()

	if path != "/" {
		t.Errorf("extractPathFromRequestURI() path = %q, want %q", path, "/")
	}
	if query != "" {
		t.Errorf("extractPathFromRequestURI() query = %q, want %q", query, "")
	}
}


// =============================================================================
// Property-Based Tests for Path Extraction
// =============================================================================

// Property 2: Path Extraction from REQUEST_URI
// *For any* REQUEST_URI string containing `go-index.cgi`, the path extractor SHALL correctly extract:
// - The path portion after `go-index.cgi` as the request path
// - The query string (if present) as separate from the path
// - An empty path SHALL be normalized to `/`
//
// **Validates: Requirements 3.3, 3.4, 3.5**

// TestProperty2_PathExtraction_PathAfterMarker tests that the path portion after
// go-index.cgi is correctly extracted as the request path.
// **Validates: Requirements 3.3**
func TestProperty2_PathExtraction_PathAfterMarker(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random CGI prefix (the part before go-index.cgi)
		cgiPrefix := rapid.StringMatching(`/cgi/[a-zA-Z0-9_/]+`).Draw(t, "cgiPrefix")

		// Generate random path segments (the part after go-index.cgi)
		pathSegments := rapid.SliceOfN(
			rapid.StringMatching(`[a-zA-Z0-9_-]+`),
			0, 5,
		).Draw(t, "pathSegments")

		// Build the path after the marker
		var pathAfterMarker string
		if len(pathSegments) > 0 {
			pathAfterMarker = "/" + strings.Join(pathSegments, "/")
		}

		// Build the full REQUEST_URI
		requestURI := cgiPrefix + cgiMarker + pathAfterMarker

		// Set the environment variable
		os.Setenv("REQUEST_URI", requestURI)
		defer os.Unsetenv("REQUEST_URI")

		// Extract path
		extractedPath, _ := extractPathFromRequestURI()

		// Property: The extracted path should match the path after the marker
		expectedPath := pathAfterMarker
		if expectedPath == "" {
			expectedPath = "/" // Empty path normalizes to "/"
		}

		if extractedPath != expectedPath {
			t.Fatalf("Path extraction failed:\n  REQUEST_URI: %q\n  Expected path: %q\n  Got path: %q",
				requestURI, expectedPath, extractedPath)
		}
	})
}

// TestProperty2_PathExtraction_QueryStringSeparation tests that query strings
// are correctly separated from the path.
// **Validates: Requirements 3.5**
func TestProperty2_PathExtraction_QueryStringSeparation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random CGI prefix
		cgiPrefix := rapid.StringMatching(`/cgi/[a-zA-Z0-9_/]+`).Draw(t, "cgiPrefix")

		// Generate random path (may be empty)
		pathSegments := rapid.SliceOfN(
			rapid.StringMatching(`[a-zA-Z0-9_-]+`),
			0, 3,
		).Draw(t, "pathSegments")

		var pathAfterMarker string
		if len(pathSegments) > 0 {
			pathAfterMarker = "/" + strings.Join(pathSegments, "/")
		}

		// Generate random query parameters
		numParams := rapid.IntRange(1, 3).Draw(t, "numParams")
		var queryParts []string
		for i := 0; i < numParams; i++ {
			key := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_]*`).Draw(t, "key")
			value := rapid.StringMatching(`[a-zA-Z0-9_/.-]+`).Draw(t, "value")
			queryParts = append(queryParts, key+"="+value)
		}
		queryString := strings.Join(queryParts, "&")

		// Build the full REQUEST_URI with query string
		requestURI := cgiPrefix + cgiMarker + pathAfterMarker + "?" + queryString

		// Set the environment variable
		os.Setenv("REQUEST_URI", requestURI)
		defer os.Unsetenv("REQUEST_URI")

		// Extract path and query
		extractedPath, extractedQuery := extractPathFromRequestURI()

		// Property 1: The extracted query should match the original query string
		if extractedQuery != queryString {
			t.Fatalf("Query string extraction failed:\n  REQUEST_URI: %q\n  Expected query: %q\n  Got query: %q",
				requestURI, queryString, extractedQuery)
		}

		// Property 2: The extracted path should NOT contain the query string
		if strings.Contains(extractedPath, "?") {
			t.Fatalf("Path should not contain '?':\n  REQUEST_URI: %q\n  Extracted path: %q",
				requestURI, extractedPath)
		}

		// Property 3: The extracted path should match the path portion
		expectedPath := pathAfterMarker
		if expectedPath == "" {
			expectedPath = "/"
		}
		if extractedPath != expectedPath {
			t.Fatalf("Path extraction with query failed:\n  REQUEST_URI: %q\n  Expected path: %q\n  Got path: %q",
				requestURI, expectedPath, extractedPath)
		}
	})
}

// TestProperty2_PathExtraction_EmptyPathNormalization tests that empty paths
// are normalized to "/".
// **Validates: Requirements 3.4**
func TestProperty2_PathExtraction_EmptyPathNormalization(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random CGI prefix
		cgiPrefix := rapid.StringMatching(`/cgi/[a-zA-Z0-9_/]+`).Draw(t, "cgiPrefix")

		// Choose whether to include a trailing slash or query-only
		variant := rapid.IntRange(0, 2).Draw(t, "variant")

		var requestURI string
		var expectedQuery string

		switch variant {
		case 0:
			// No path, no query: /cgi/.../go-index.cgi
			requestURI = cgiPrefix + cgiMarker
			expectedQuery = ""
		case 1:
			// Trailing slash only: /cgi/.../go-index.cgi/
			requestURI = cgiPrefix + cgiMarker + "/"
			expectedQuery = ""
		case 2:
			// Query only, no path: /cgi/.../go-index.cgi?key=value
			key := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_]*`).Draw(t, "key")
			value := rapid.StringMatching(`[a-zA-Z0-9_]+`).Draw(t, "value")
			expectedQuery = key + "=" + value
			requestURI = cgiPrefix + cgiMarker + "?" + expectedQuery
		}

		// Set the environment variable
		os.Setenv("REQUEST_URI", requestURI)
		defer os.Unsetenv("REQUEST_URI")

		// Extract path and query
		extractedPath, extractedQuery := extractPathFromRequestURI()

		// Property: Empty path should be normalized to "/"
		if extractedPath != "/" {
			t.Fatalf("Empty path not normalized to '/':\n  REQUEST_URI: %q\n  Expected path: %q\n  Got path: %q",
				requestURI, "/", extractedPath)
		}

		// Verify query string is correctly extracted
		if extractedQuery != expectedQuery {
			t.Fatalf("Query extraction failed for empty path case:\n  REQUEST_URI: %q\n  Expected query: %q\n  Got query: %q",
				requestURI, expectedQuery, extractedQuery)
		}
	})
}

// TestProperty2_PathExtraction_Roundtrip tests that the extracted path and query
// can be used to reconstruct the portion after the CGI marker.
// **Validates: Requirements 3.3, 3.4, 3.5**
func TestProperty2_PathExtraction_Roundtrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random CGI prefix
		cgiPrefix := rapid.StringMatching(`/cgi/[a-zA-Z0-9_/]+`).Draw(t, "cgiPrefix")

		// Generate random path segments
		pathSegments := rapid.SliceOfN(
			rapid.StringMatching(`[a-zA-Z0-9_-]+`),
			0, 4,
		).Draw(t, "pathSegments")

		var pathAfterMarker string
		if len(pathSegments) > 0 {
			pathAfterMarker = "/" + strings.Join(pathSegments, "/")
		}

		// Optionally add query string
		hasQuery := rapid.Bool().Draw(t, "hasQuery")
		var queryString string
		if hasQuery {
			key := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_]*`).Draw(t, "key")
			value := rapid.StringMatching(`[a-zA-Z0-9_/.-]+`).Draw(t, "value")
			queryString = key + "=" + value
		}

		// Build the full REQUEST_URI
		requestURI := cgiPrefix + cgiMarker + pathAfterMarker
		if queryString != "" {
			requestURI += "?" + queryString
		}

		// Set the environment variable
		os.Setenv("REQUEST_URI", requestURI)
		defer os.Unsetenv("REQUEST_URI")

		// Extract path and query
		extractedPath, extractedQuery := extractPathFromRequestURI()

		// Reconstruct the portion after the marker
		var reconstructed string
		if extractedPath == "/" && extractedQuery == "" {
			// Special case: empty path with no query
			reconstructed = ""
		} else if extractedPath == "/" && extractedQuery != "" {
			// Path is "/" with query
			reconstructed = "?" + extractedQuery
		} else if extractedQuery != "" {
			reconstructed = extractedPath + "?" + extractedQuery
		} else {
			reconstructed = extractedPath
		}

		// Build expected portion after marker
		expected := pathAfterMarker
		if queryString != "" {
			if expected == "" {
				expected = "?" + queryString
			} else {
				expected += "?" + queryString
			}
		}

		// Property: Reconstructed should match the original portion after marker
		if reconstructed != expected {
			t.Fatalf("Roundtrip failed:\n  REQUEST_URI: %q\n  Expected after marker: %q\n  Reconstructed: %q\n  Path: %q, Query: %q",
				requestURI, expected, reconstructed, extractedPath, extractedQuery)
		}
	})
}

// TestProperty2_PathExtraction_NoMarker tests that when the CGI marker is not present,
// the function returns default values.
// **Validates: Requirements 3.3, 3.4**
func TestProperty2_PathExtraction_NoMarker(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random URI that does NOT contain the CGI marker
		// Use a pattern that won't accidentally include "go-index.cgi"
		randomPath := rapid.StringMatching(`/[a-z]+/[a-z]+/[a-z]+`).Draw(t, "randomPath")

		// Ensure it doesn't contain the marker (defensive check)
		if strings.Contains(randomPath, cgiMarker) {
			t.Skip("Generated path accidentally contains marker")
		}

		// Set the environment variable
		os.Setenv("REQUEST_URI", randomPath)
		defer os.Unsetenv("REQUEST_URI")

		// Extract path and query
		extractedPath, extractedQuery := extractPathFromRequestURI()

		// Property: When marker is not found, path should be "/" and query should be empty
		if extractedPath != "/" {
			t.Fatalf("Path should be '/' when marker not found:\n  REQUEST_URI: %q\n  Got path: %q",
				randomPath, extractedPath)
		}
		if extractedQuery != "" {
			t.Fatalf("Query should be empty when marker not found:\n  REQUEST_URI: %q\n  Got query: %q",
				randomPath, extractedQuery)
		}
	})
}


// =============================================================================
// Unit Tests for CGI Prefix Extraction (Task 5.4)
// =============================================================================

// TestExtractCGIPrefix tests the CGI prefix extraction logic
// Requirements: 3.2, 3.3
func TestExtractCGIPrefix(t *testing.T) {
	tests := []struct {
		name           string
		requestURI     string
		expectedPrefix string
	}{
		// Standard index.cgi marker
		{
			name:           "standard index.cgi path",
			requestURI:     "/cgi/ThirdParty/onlyoffice-fnos/index.cgi/editor?path=/vol1/test.docx",
			expectedPrefix: "/cgi/ThirdParty/onlyoffice-fnos/index.cgi",
		},
		{
			name:           "index.cgi with doc-svr path",
			requestURI:     "/cgi/ThirdParty/onlyoffice-fnos/index.cgi/doc-svr/web-apps/apps/api/documents/api.js",
			expectedPrefix: "/cgi/ThirdParty/onlyoffice-fnos/index.cgi",
		},
		{
			name:           "index.cgi without trailing path",
			requestURI:     "/cgi/ThirdParty/onlyoffice-fnos/index.cgi",
			expectedPrefix: "/cgi/ThirdParty/onlyoffice-fnos/index.cgi",
		},
		{
			name:           "index.cgi with trailing slash",
			requestURI:     "/cgi/ThirdParty/onlyoffice-fnos/index.cgi/",
			expectedPrefix: "/cgi/ThirdParty/onlyoffice-fnos/index.cgi",
		},
		// go-index.cgi marker (primary)
		{
			name:           "go-index.cgi path",
			requestURI:     "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi/editor?path=/vol1/test.docx",
			expectedPrefix: "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi",
		},
		{
			name:           "go-index.cgi with doc-svr",
			requestURI:     "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi/doc-svr/api.js",
			expectedPrefix: "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi",
		},
		// Edge cases
		{
			name:           "no CGI marker",
			requestURI:     "/some/other/path",
			expectedPrefix: "",
		},
		{
			name:           "empty REQUEST_URI",
			requestURI:     "",
			expectedPrefix: "",
		},
		{
			name:           "query string only",
			requestURI:     "/cgi/ThirdParty/onlyoffice-fnos/index.cgi?foo=bar",
			expectedPrefix: "/cgi/ThirdParty/onlyoffice-fnos/index.cgi",
		},
		// Different CGI paths
		{
			name:           "short CGI path",
			requestURI:     "/cgi/index.cgi/editor",
			expectedPrefix: "/cgi/index.cgi",
		},
		{
			name:           "nested CGI path",
			requestURI:     "/apps/cgi-bin/ThirdParty/onlyoffice/index.cgi/doc-svr",
			expectedPrefix: "/apps/cgi-bin/ThirdParty/onlyoffice/index.cgi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the REQUEST_URI environment variable
			os.Setenv("REQUEST_URI", tt.requestURI)
			defer os.Unsetenv("REQUEST_URI")

			prefix := extractCGIPrefix()

			if prefix != tt.expectedPrefix {
				t.Errorf("extractCGIPrefix() = %q, want %q", prefix, tt.expectedPrefix)
			}
		})
	}
}

// TestExtractCGIPrefix_NoEnvVar tests behavior when REQUEST_URI is not set
func TestExtractCGIPrefix_NoEnvVar(t *testing.T) {
	// Ensure REQUEST_URI is not set
	os.Unsetenv("REQUEST_URI")

	prefix := extractCGIPrefix()

	if prefix != "" {
		t.Errorf("extractCGIPrefix() = %q, want empty string", prefix)
	}
}

// TestBuildDocServerPath tests the DOC_SERVER_PATH construction
// Requirements: 3.2, 3.3
func TestBuildDocServerPath(t *testing.T) {
	tests := []struct {
		name         string
		httpHost     string
		cgiPrefix    string
		expectedPath string
	}{
		// Standard cases
		{
			name:         "internal IP with port",
			httpHost:     "192.168.1.177:5666",
			cgiPrefix:    "/cgi/ThirdParty/onlyoffice-fnos/index.cgi",
			expectedPath: "192.168.1.177:5666/cgi/ThirdParty/onlyoffice-fnos/index.cgi/doc-svr",
		},
		{
			name:         "external domain",
			httpHost:     "example.com",
			cgiPrefix:    "/cgi/ThirdParty/onlyoffice-fnos/index.cgi",
			expectedPath: "example.com/cgi/ThirdParty/onlyoffice-fnos/index.cgi/doc-svr",
		},
		{
			name:         "external domain with port",
			httpHost:     "example.com:8080",
			cgiPrefix:    "/cgi/ThirdParty/onlyoffice-fnos/index.cgi",
			expectedPath: "example.com:8080/cgi/ThirdParty/onlyoffice-fnos/index.cgi/doc-svr",
		},
		{
			name:         "go-index.cgi prefix",
			httpHost:     "192.168.1.100:5666",
			cgiPrefix:    "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi",
			expectedPath: "192.168.1.100:5666/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi/doc-svr",
		},
		// Edge cases - empty inputs
		{
			name:         "empty httpHost",
			httpHost:     "",
			cgiPrefix:    "/cgi/ThirdParty/onlyoffice-fnos/index.cgi",
			expectedPath: "",
		},
		{
			name:         "empty cgiPrefix",
			httpHost:     "192.168.1.177:5666",
			cgiPrefix:    "",
			expectedPath: "",
		},
		{
			name:         "both empty",
			httpHost:     "",
			cgiPrefix:    "",
			expectedPath: "",
		},
		// Short paths
		{
			name:         "short CGI prefix",
			httpHost:     "localhost:5666",
			cgiPrefix:    "/cgi/index.cgi",
			expectedPath: "localhost:5666/cgi/index.cgi/doc-svr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildDocServerPath(tt.httpHost, tt.cgiPrefix)

			if result != tt.expectedPath {
				t.Errorf("buildDocServerPath(%q, %q) = %q, want %q",
					tt.httpHost, tt.cgiPrefix, result, tt.expectedPath)
			}
		})
	}
}

// TestGetHTTPHost tests the HTTP host extraction from CGI environment
func TestGetHTTPHost(t *testing.T) {
	tests := []struct {
		name         string
		httpHost     string
		serverName   string
		serverPort   string
		expectedHost string
	}{
		// HTTP_HOST takes precedence
		{
			name:         "HTTP_HOST with port",
			httpHost:     "192.168.1.177:5666",
			serverName:   "ignored",
			serverPort:   "ignored",
			expectedHost: "192.168.1.177:5666",
		},
		{
			name:         "HTTP_HOST without port",
			httpHost:     "example.com",
			serverName:   "ignored",
			serverPort:   "ignored",
			expectedHost: "example.com",
		},
		// Fall back to SERVER_NAME:SERVER_PORT
		{
			name:         "SERVER_NAME with non-standard port",
			httpHost:     "",
			serverName:   "192.168.1.177",
			serverPort:   "5666",
			expectedHost: "192.168.1.177:5666",
		},
		{
			name:         "SERVER_NAME with port 80",
			httpHost:     "",
			serverName:   "example.com",
			serverPort:   "80",
			expectedHost: "example.com",
		},
		{
			name:         "SERVER_NAME with port 443",
			httpHost:     "",
			serverName:   "example.com",
			serverPort:   "443",
			expectedHost: "example.com",
		},
		{
			name:         "SERVER_NAME without port",
			httpHost:     "",
			serverName:   "example.com",
			serverPort:   "",
			expectedHost: "example.com",
		},
		// Edge cases
		{
			name:         "no environment variables",
			httpHost:     "",
			serverName:   "",
			serverPort:   "",
			expectedHost: "",
		},
		{
			name:         "only SERVER_PORT (no SERVER_NAME)",
			httpHost:     "",
			serverName:   "",
			serverPort:   "5666",
			expectedHost: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant environment variables first
			os.Unsetenv("HTTP_HOST")
			os.Unsetenv("SERVER_NAME")
			os.Unsetenv("SERVER_PORT")

			// Set environment variables as specified
			if tt.httpHost != "" {
				os.Setenv("HTTP_HOST", tt.httpHost)
			}
			if tt.serverName != "" {
				os.Setenv("SERVER_NAME", tt.serverName)
			}
			if tt.serverPort != "" {
				os.Setenv("SERVER_PORT", tt.serverPort)
			}

			defer func() {
				os.Unsetenv("HTTP_HOST")
				os.Unsetenv("SERVER_NAME")
				os.Unsetenv("SERVER_PORT")
			}()

			result := getHTTPHost()

			if result != tt.expectedHost {
				t.Errorf("getHTTPHost() = %q, want %q", result, tt.expectedHost)
			}
		})
	}
}

// TestCGIPrefixIntegration tests the full integration of CGI prefix extraction
// and DOC_SERVER_PATH construction
func TestCGIPrefixIntegration(t *testing.T) {
	tests := []struct {
		name               string
		requestURI         string
		httpHost           string
		expectedDocSvrPath string
	}{
		{
			name:               "internal network access",
			requestURI:         "/cgi/ThirdParty/onlyoffice-fnos/index.cgi/editor?path=/vol1/test.docx",
			httpHost:           "192.168.1.177:5666",
			expectedDocSvrPath: "192.168.1.177:5666/cgi/ThirdParty/onlyoffice-fnos/index.cgi/doc-svr",
		},
		{
			name:               "external domain access",
			requestURI:         "/cgi/ThirdParty/onlyoffice-fnos/index.cgi/editor?path=/vol1/test.docx",
			httpHost:           "example.com",
			expectedDocSvrPath: "example.com/cgi/ThirdParty/onlyoffice-fnos/index.cgi/doc-svr",
		},
		{
			name:               "go-index.cgi marker",
			requestURI:         "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi/doc-svr/api.js",
			httpHost:           "192.168.1.100:5666",
			expectedDocSvrPath: "192.168.1.100:5666/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi/doc-svr",
		},
		{
			name:               "no CGI marker - empty result",
			requestURI:         "/some/other/path",
			httpHost:           "192.168.1.177:5666",
			expectedDocSvrPath: "",
		},
		{
			name:               "no HTTP host - empty result",
			requestURI:         "/cgi/ThirdParty/onlyoffice-fnos/index.cgi/editor",
			httpHost:           "",
			expectedDocSvrPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			os.Setenv("REQUEST_URI", tt.requestURI)
			if tt.httpHost != "" {
				os.Setenv("HTTP_HOST", tt.httpHost)
			} else {
				os.Unsetenv("HTTP_HOST")
			}
			os.Unsetenv("SERVER_NAME")
			os.Unsetenv("SERVER_PORT")

			defer func() {
				os.Unsetenv("REQUEST_URI")
				os.Unsetenv("HTTP_HOST")
			}()

			// Extract CGI prefix and HTTP host
			cgiPrefix := extractCGIPrefix()
			httpHost := getHTTPHost()

			// Build DOC_SERVER_PATH
			docServerPath := buildDocServerPath(httpHost, cgiPrefix)

			if docServerPath != tt.expectedDocSvrPath {
				t.Errorf("Integration test failed:\n  REQUEST_URI: %q\n  HTTP_HOST: %q\n  Expected: %q\n  Got: %q",
					tt.requestURI, tt.httpHost, tt.expectedDocSvrPath, docServerPath)
			}
		})
	}
}


// =============================================================================
// Unit Tests for Configuration Priority (Task 7.1)
// =============================================================================

// TestResolveBaseURL tests the base URL resolution with priority:
// 1. Command line flag (highest priority)
// 2. Environment variable
// 3. Default value (lowest priority)
//
// Requirements: 5.1, 5.3
func TestResolveBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		flagValue   string
		envValue    string
		port        string
		expectedURL string
	}{
		// Priority 1: Command line flag takes precedence
		{
			name:        "flag takes precedence over env and default",
			flagValue:   "http://flag.example.com:8080",
			envValue:    "http://env.example.com:9090",
			port:        "10099",
			expectedURL: "http://flag.example.com:8080",
		},
		{
			name:        "flag takes precedence over default (no env)",
			flagValue:   "http://flag.example.com:8080",
			envValue:    "",
			port:        "10099",
			expectedURL: "http://flag.example.com:8080",
		},
		// Priority 2: Environment variable when flag is not provided
		{
			name:        "env used when flag is empty",
			flagValue:   "",
			envValue:    "http://env.example.com:9090",
			port:        "10099",
			expectedURL: "http://env.example.com:9090",
		},
		// Priority 3: Default value when both flag and env are empty
		{
			name:        "default used when flag and env are empty",
			flagValue:   "",
			envValue:    "",
			port:        "10099",
			expectedURL: "http://localhost:10099",
		},
		{
			name:        "default with custom port",
			flagValue:   "",
			envValue:    "",
			port:        "8080",
			expectedURL: "http://localhost:8080",
		},
		// Edge cases
		{
			name:        "flag with different port than default",
			flagValue:   "http://192.168.1.100:3000",
			envValue:    "http://192.168.1.100:4000",
			port:        "5000",
			expectedURL: "http://192.168.1.100:3000",
		},
		{
			name:        "env with https",
			flagValue:   "",
			envValue:    "https://secure.example.com",
			port:        "10099",
			expectedURL: "https://secure.example.com",
		},
		{
			name:        "flag with https overrides env",
			flagValue:   "https://flag-secure.example.com",
			envValue:    "http://env.example.com",
			port:        "10099",
			expectedURL: "https://flag-secure.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveBaseURL(tt.flagValue, tt.envValue, tt.port)

			if result != tt.expectedURL {
				t.Errorf("resolveBaseURL(%q, %q, %q) = %q, want %q",
					tt.flagValue, tt.envValue, tt.port, result, tt.expectedURL)
			}
		})
	}
}

// TestResolveBaseURL_FlagPrecedence specifically tests that command line flag
// always takes precedence over environment variable (Requirement 5.3)
func TestResolveBaseURL_FlagPrecedence(t *testing.T) {
	// When both flag and env are set, flag should always win
	flagValue := "http://from-flag.example.com"
	envValue := "http://from-env.example.com"
	port := "10099"

	result := resolveBaseURL(flagValue, envValue, port)

	if result != flagValue {
		t.Errorf("Flag should take precedence: got %q, want %q", result, flagValue)
	}
}

// TestResolveBaseURL_EnvFallback specifically tests that environment variable
// is used when flag is not provided (Requirement 5.3)
func TestResolveBaseURL_EnvFallback(t *testing.T) {
	// When flag is empty, env should be used
	flagValue := ""
	envValue := "http://from-env.example.com"
	port := "10099"

	result := resolveBaseURL(flagValue, envValue, port)

	if result != envValue {
		t.Errorf("Env should be used when flag is empty: got %q, want %q", result, envValue)
	}
}

// TestResolveBaseURL_DefaultFallback specifically tests that default value
// is used when both flag and env are not provided
func TestResolveBaseURL_DefaultFallback(t *testing.T) {
	// When both flag and env are empty, default should be used
	flagValue := ""
	envValue := ""
	port := "12345"

	result := resolveBaseURL(flagValue, envValue, port)

	expected := "http://localhost:12345"
	if result != expected {
		t.Errorf("Default should be used when flag and env are empty: got %q, want %q", result, expected)
	}
}

// =============================================================================
// Property-Based Tests for Configuration Priority (Task 7.2)
// =============================================================================

// Property 3: Configuration Priority
// *For any* combination of environment variables and command line flags, the configuration loader SHALL follow this priority:
// - Command line `--base-url` flag takes precedence over `BASE_URL` environment variable
// - `BASE_URL` environment variable is used when flag is not provided
// - All existing environment variables (`DOCUMENT_SERVER_URL`, `DOCUMENT_SERVER_SECRET`, etc.) continue to work
//
// **Validates: Requirements 5.1, 5.3**

// urlGenerator generates valid URL strings for testing
func urlGenerator() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		// Generate protocol
		protocol := rapid.SampledFrom([]string{"http", "https"}).Draw(t, "protocol")

		// Generate host (IP or domain)
		hostType := rapid.IntRange(0, 1).Draw(t, "hostType")
		var host string
		if hostType == 0 {
			// IP address
			ip1 := rapid.IntRange(1, 255).Draw(t, "ip1")
			ip2 := rapid.IntRange(0, 255).Draw(t, "ip2")
			ip3 := rapid.IntRange(0, 255).Draw(t, "ip3")
			ip4 := rapid.IntRange(1, 254).Draw(t, "ip4")
			host = fmt.Sprintf("%d.%d.%d.%d", ip1, ip2, ip3, ip4)
		} else {
			// Domain name
			subdomain := rapid.StringMatching(`[a-z][a-z0-9-]{0,10}`).Draw(t, "subdomain")
			tld := rapid.SampledFrom([]string{"com", "org", "net", "io", "local"}).Draw(t, "tld")
			host = subdomain + "." + tld
		}

		// Generate port
		port := rapid.IntRange(1024, 65535).Draw(t, "port")

		return fmt.Sprintf("%s://%s:%d", protocol, host, port)
	})
}

// portGenerator generates valid port strings for testing
func portGenerator() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		port := rapid.IntRange(1024, 65535).Draw(t, "port")
		return fmt.Sprintf("%d", port)
	})
}

// TestProperty3_ConfigPriority_FlagTakesPrecedence tests that command line flag
// always takes precedence over environment variable.
// **Validates: Requirements 5.3**
func TestProperty3_ConfigPriority_FlagTakesPrecedence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random flag URL
		flagURL := urlGenerator().Draw(t, "flagURL")

		// Generate random env URL (different from flag)
		envURL := urlGenerator().Draw(t, "envURL")

		// Generate random port
		port := portGenerator().Draw(t, "port")

		// Call resolveBaseURL
		result := resolveBaseURL(flagURL, envURL, port)

		// Property: When flag is provided, it should always be returned
		if result != flagURL {
			t.Fatalf("Flag should take precedence:\n  Flag URL: %q\n  Env URL: %q\n  Port: %q\n  Expected: %q\n  Got: %q",
				flagURL, envURL, port, flagURL, result)
		}
	})
}

// TestProperty3_ConfigPriority_EnvUsedWhenNoFlag tests that environment variable
// is used when command line flag is not provided.
// **Validates: Requirements 5.1, 5.3**
func TestProperty3_ConfigPriority_EnvUsedWhenNoFlag(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Flag is empty (not provided)
		flagURL := ""

		// Generate random env URL
		envURL := urlGenerator().Draw(t, "envURL")

		// Generate random port
		port := portGenerator().Draw(t, "port")

		// Call resolveBaseURL
		result := resolveBaseURL(flagURL, envURL, port)

		// Property: When flag is empty and env is provided, env should be returned
		if result != envURL {
			t.Fatalf("Env should be used when flag is empty:\n  Flag URL: %q\n  Env URL: %q\n  Port: %q\n  Expected: %q\n  Got: %q",
				flagURL, envURL, port, envURL, result)
		}
	})
}

// TestProperty3_ConfigPriority_DefaultWhenBothEmpty tests that default value
// is used when both flag and environment variable are empty.
// **Validates: Requirements 5.1, 5.3**
func TestProperty3_ConfigPriority_DefaultWhenBothEmpty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Both flag and env are empty
		flagURL := ""
		envURL := ""

		// Generate random port
		port := portGenerator().Draw(t, "port")

		// Call resolveBaseURL
		result := resolveBaseURL(flagURL, envURL, port)

		// Property: When both flag and env are empty, default should be returned
		expected := fmt.Sprintf("http://localhost:%s", port)
		if result != expected {
			t.Fatalf("Default should be used when both are empty:\n  Flag URL: %q\n  Env URL: %q\n  Port: %q\n  Expected: %q\n  Got: %q",
				flagURL, envURL, port, expected, result)
		}
	})
}

// TestProperty3_ConfigPriority_PriorityChain tests the complete priority chain:
// flag > env > default
// **Validates: Requirements 5.1, 5.3**
func TestProperty3_ConfigPriority_PriorityChain(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random values
		flagURL := urlGenerator().Draw(t, "flagURL")
		envURL := urlGenerator().Draw(t, "envURL")
		port := portGenerator().Draw(t, "port")

		// Choose which values to provide (0=none, 1=env only, 2=flag only, 3=both)
		scenario := rapid.IntRange(0, 3).Draw(t, "scenario")

		var actualFlag, actualEnv string
		var expected string

		switch scenario {
		case 0:
			// Neither flag nor env provided -> default
			actualFlag = ""
			actualEnv = ""
			expected = fmt.Sprintf("http://localhost:%s", port)
		case 1:
			// Only env provided -> env
			actualFlag = ""
			actualEnv = envURL
			expected = envURL
		case 2:
			// Only flag provided -> flag
			actualFlag = flagURL
			actualEnv = ""
			expected = flagURL
		case 3:
			// Both provided -> flag (highest priority)
			actualFlag = flagURL
			actualEnv = envURL
			expected = flagURL
		}

		result := resolveBaseURL(actualFlag, actualEnv, port)

		if result != expected {
			t.Fatalf("Priority chain failed for scenario %d:\n  Flag: %q\n  Env: %q\n  Port: %q\n  Expected: %q\n  Got: %q",
				scenario, actualFlag, actualEnv, port, expected, result)
		}
	})
}

// TestProperty3_ConfigPriority_ExistingEnvVarsContinueToWork tests that existing
// environment variables (DOCUMENT_SERVER_URL, DOCUMENT_SERVER_SECRET, etc.) continue to work.
// **Validates: Requirements 5.1**
func TestProperty3_ConfigPriority_ExistingEnvVarsContinueToWork(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random values for existing environment variables
		docServerURL := urlGenerator().Draw(t, "docServerURL")
		docServerSecret := rapid.StringMatching(`[a-zA-Z0-9]{16,32}`).Draw(t, "docServerSecret")
		baseURL := urlGenerator().Draw(t, "baseURL")
		docServerPath := rapid.StringMatching(`[a-z0-9.-]+:[0-9]+/cgi/[a-zA-Z0-9_/]+/index\.cgi/doc-svr`).Draw(t, "docServerPath")

		// Set environment variables
		os.Setenv("DOCUMENT_SERVER_URL", docServerURL)
		os.Setenv("DOCUMENT_SERVER_SECRET", docServerSecret)
		os.Setenv("BASE_URL", baseURL)
		os.Setenv("DOC_SERVER_PATH", docServerPath)

		defer func() {
			os.Unsetenv("DOCUMENT_SERVER_URL")
			os.Unsetenv("DOCUMENT_SERVER_SECRET")
			os.Unsetenv("BASE_URL")
			os.Unsetenv("DOC_SERVER_PATH")
		}()

		// Verify environment variables are correctly set and readable
		if got := os.Getenv("DOCUMENT_SERVER_URL"); got != docServerURL {
			t.Fatalf("DOCUMENT_SERVER_URL not set correctly:\n  Expected: %q\n  Got: %q", docServerURL, got)
		}
		if got := os.Getenv("DOCUMENT_SERVER_SECRET"); got != docServerSecret {
			t.Fatalf("DOCUMENT_SERVER_SECRET not set correctly:\n  Expected: %q\n  Got: %q", docServerSecret, got)
		}
		if got := os.Getenv("BASE_URL"); got != baseURL {
			t.Fatalf("BASE_URL not set correctly:\n  Expected: %q\n  Got: %q", baseURL, got)
		}
		if got := os.Getenv("DOC_SERVER_PATH"); got != docServerPath {
			t.Fatalf("DOC_SERVER_PATH not set correctly:\n  Expected: %q\n  Got: %q", docServerPath, got)
		}

		// Test that resolveBaseURL correctly uses BASE_URL when flag is empty
		port := portGenerator().Draw(t, "port")
		result := resolveBaseURL("", baseURL, port)

		if result != baseURL {
			t.Fatalf("BASE_URL env var should be used when flag is empty:\n  BASE_URL: %q\n  Port: %q\n  Expected: %q\n  Got: %q",
				baseURL, port, baseURL, result)
		}
	})
}

// TestProperty3_ConfigPriority_FlagOverridesEnvInAllCases tests that flag always
// overrides env regardless of the URL format or content.
// **Validates: Requirements 5.3**
func TestProperty3_ConfigPriority_FlagOverridesEnvInAllCases(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate various URL formats
		urlFormats := []string{
			"http://localhost:%d",
			"https://localhost:%d",
			"http://127.0.0.1:%d",
			"https://192.168.1.100:%d",
			"http://example.com:%d",
			"https://secure.example.com:%d",
		}

		flagFormat := rapid.SampledFrom(urlFormats).Draw(t, "flagFormat")
		envFormat := rapid.SampledFrom(urlFormats).Draw(t, "envFormat")

		flagPort := rapid.IntRange(1024, 65535).Draw(t, "flagPort")
		envPort := rapid.IntRange(1024, 65535).Draw(t, "envPort")
		defaultPort := rapid.IntRange(1024, 65535).Draw(t, "defaultPort")

		flagURL := fmt.Sprintf(flagFormat, flagPort)
		envURL := fmt.Sprintf(envFormat, envPort)
		portStr := fmt.Sprintf("%d", defaultPort)

		result := resolveBaseURL(flagURL, envURL, portStr)

		// Property: Flag should always win, regardless of URL format
		if result != flagURL {
			t.Fatalf("Flag should override env in all cases:\n  Flag URL: %q\n  Env URL: %q\n  Port: %q\n  Expected: %q\n  Got: %q",
				flagURL, envURL, portStr, flagURL, result)
		}
	})
}
