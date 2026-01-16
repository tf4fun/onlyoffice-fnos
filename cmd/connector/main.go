package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"onlyoffice-fnos/internal/config"
	"onlyoffice-fnos/internal/file"
	"onlyoffice-fnos/internal/format"
	"onlyoffice-fnos/internal/jwt"
	"onlyoffice-fnos/internal/server"
)

const (
	// Mode constants
	ModeServer = "server"
	ModeCGI    = "cgi"

	// Default values
	defaultMode = ModeCGI
	defaultPort = "10099"

	// Server timeouts
	shutdownTimeout = 10 * time.Second
	readTimeout     = 30 * time.Second
	writeTimeout    = 60 * time.Second
	idleTimeout     = 120 * time.Second
)

// Config holds the command line configuration
type Config struct {
	Mode    string // "server" or "cgi", default "cgi"
	Port    string // Server mode listening port, default "10099"
	BaseURL string // Callback base URL
}

func main() {
	config, err := parseFlags(os.Args[1:])
	if err != nil {
		log.Fatalf("Error parsing flags: %v", err)
	}

	switch config.Mode {
	case ModeServer:
		runServerMode(config)
	case ModeCGI:
		runCGIMode(config)
	default:
		// This should never happen due to validation in parseFlags
		log.Fatalf("Invalid mode: %s", config.Mode)
	}
}

// parseFlags parses command line arguments and returns a Config
// It validates the mode value and returns an error for invalid modes
func parseFlags(args []string) (*Config, error) {
	fs := flag.NewFlagSet("connector", flag.ContinueOnError)

	mode := fs.String("mode", defaultMode, "Running mode: 'server' or 'cgi' (default: cgi)")
	port := fs.String("port", defaultPort, "HTTP server port for server mode (default: 10099)")
	baseURL := fs.String("base-url", "", "Base URL for callbacks (e.g., http://192.168.1.100:10099)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// Validate mode
	if *mode != ModeServer && *mode != ModeCGI {
		return nil, fmt.Errorf("invalid mode '%s': must be '%s' or '%s'", *mode, ModeServer, ModeCGI)
	}

	return &Config{
		Mode:    *mode,
		Port:    *port,
		BaseURL: *baseURL,
	}, nil
}

// resolveBaseURL determines the base URL with the following priority:
// 1. Command line flag (flagValue) - highest priority
// 2. Environment variable (envValue) - from BASE_URL env var
// 3. Default value based on port - lowest priority
//
// This ensures command line arguments take precedence over environment variables.
// Requirements: 5.1, 5.3
func resolveBaseURL(flagValue, envValue, port string) string {
	// Priority 1: Command line flag takes precedence
	if flagValue != "" {
		return flagValue
	}

	// Priority 2: Environment variable
	if envValue != "" {
		return envValue
	}

	// Priority 3: Default value
	return fmt.Sprintf("http://localhost:%s", port)
}

// runServerMode starts the connector in standalone HTTP server mode
// This migrates the logic from cmd/server/main.go
// Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 5.1, 5.3
func runServerMode(cfg *Config) {
	// Load settings from environment variables first (Requirement 5.1)
	settings, err := config.LoadFromEnv()
	if err != nil {
		log.Printf("Warning: %v, using defaults", err)
		settings = &config.Settings{}
	}

	// Determine base URL with priority: command line flag > env var > default
	// (Requirements 2.3, 5.3)
	baseURL := resolveBaseURL(cfg.BaseURL, settings.BaseURL, cfg.Port)

	// Log startup information (Requirement 2.5)
	log.Printf("OnlyOffice fnOS Connector starting in server mode...")
	log.Printf("  Port: %s", cfg.Port)
	log.Printf("  Base URL: %s", baseURL)

	if settings.DocumentServerURL != "" {
		log.Printf("  Document Server URL: %s", settings.DocumentServerURL)
	}
	if settings.DocumentServerSecret != "" {
		log.Printf("  JWT Secret: configured")
	}
	if settings.BaseURL != "" && cfg.BaseURL != "" && cfg.BaseURL != settings.BaseURL {
		log.Printf("  Note: --base-url flag overrides BASE_URL env var")
	}

	// Initialize modules
	formatManager := format.NewManager()
	jwtManager := jwt.NewManager()
	fileService := file.NewService("", 0) // No base path restriction, no size limit

	// Create server configuration
	serverConfig := &server.Config{
		Settings:      settings,
		FileService:   fileService,
		FormatManager: formatManager,
		JWTManager:    jwtManager,
		BaseURL:       baseURL,
	}

	// Create HTTP server (Requirement 2.1)
	srv := server.New(serverConfig)

	// Create HTTP server with timeouts (Requirement 2.2 - port support)
	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      srv,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	// Channel to listen for errors from server
	serverErrors := make(chan error, 1)

	// Start server in goroutine
	go func() {
		log.Printf("Server listening on :%s", cfg.Port)
		serverErrors <- httpServer.ListenAndServe()
	}()

	// Channel to listen for interrupt signals (Requirement 2.4)
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or server error
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	case sig := <-shutdown:
		log.Printf("Received signal %v, shutting down...", sig)

		// Create context with timeout for graceful shutdown (Requirement 2.4)
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		// Attempt graceful shutdown
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("Graceful shutdown failed: %v", err)
			// Force close
			if err := httpServer.Close(); err != nil {
				log.Printf("Force close failed: %v", err)
			}
		}
	}

	log.Println("Server stopped")
}

// runCGIMode starts the connector in CGI mode
// This implements the CGI adapter wrapping internal/server
// Requirements: 3.1, 3.6, 5.1, 5.2, 5.3
func runCGIMode(cfg *Config) {
	// Load settings from environment variables first (Requirement 5.1)
	settings, err := config.LoadFromEnv()
	if err != nil {
		log.Printf("Warning: %v, using defaults", err)
		settings = &config.Settings{}
	}

	// Determine base URL with priority: command line flag > env var > default
	// In CGI mode, BASE_URL should be set via environment variable
	// pointing to the internal server address for callbacks
	// (Requirements 5.3)
	baseURL := resolveBaseURL(cfg.BaseURL, settings.BaseURL, defaultPort)

	// If DocServerPath is not set, construct it from CGI environment
	// This is used by the frontend to access Document Server through the CGI proxy
	// Requirements: 3.2, 3.3, 5.2
	if settings.DocServerPath == "" {
		httpHost := getHTTPHost()
		cgiPrefix := extractCGIPrefix()
		if docServerPath := buildDocServerPath(httpHost, cgiPrefix); docServerPath != "" {
			settings.DocServerPath = docServerPath
			log.Printf("CGI mode: DocServerPath derived from environment: %s", docServerPath)
		}
	}

	// Initialize modules
	formatManager := format.NewManager()
	jwtManager := jwt.NewManager()
	fileService := file.NewService("", 0) // No base path restriction, no size limit

	// Create server configuration
	serverConfig := &server.Config{
		Settings:      settings,
		FileService:   fileService,
		FormatManager: formatManager,
		JWTManager:    jwtManager,
		BaseURL:       baseURL,
	}

	// Create the server instance (Requirement 3.1 - run as CGI handler)
	srv := server.New(serverConfig)

	// Create CGI adapter that wraps the server
	handler := &cgiAdapter{server: srv}

	// Serve requests through CGI interface (Requirement 3.6)
	if err := cgi.Serve(handler); err != nil {
		log.Printf("CGI serve error: %v", err)
	}
}

// cgiAdapter wraps a server.Server to adapt it for CGI mode
// It handles path extraction from REQUEST_URI environment variable
type cgiAdapter struct {
	server *server.Server
}

// ServeHTTP implements http.Handler for the CGI adapter
// It extracts the actual path from REQUEST_URI and delegates to the wrapped server
func (a *cgiAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract path and query from REQUEST_URI (Requirements 3.2, 3.3, 3.4, 3.5)
	path, query := extractPathFromRequestURI()

	// Update the request URL with extracted path and query
	r.URL.Path = path
	r.URL.RawQuery = query

	// Delegate to the wrapped server
	a.server.ServeHTTP(w, r)
}

// cgiMarker is the marker used to identify the CGI script in the REQUEST_URI
const cgiMarker = "go-index.cgi"

// indexCGIMarker is the alternative marker for index.cgi
const indexCGIMarker = "index.cgi"

// extractPathFromRequestURI extracts the request path and query string from REQUEST_URI
// It finds the CGI marker (go-index.cgi) and extracts the path after it.
//
// Requirements:
// - 3.2: Read REQUEST_URI environment variable
// - 3.3: Extract path after go-index.cgi
// - 3.4: Use "/" as default for empty path
// - 3.5: Correctly parse and separate path and query parameters
//
// Examples:
// - "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi/editor?path=/vol1/test.docx"
//   -> path="/editor", query="path=/vol1/test.docx"
// - "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi"
//   -> path="/", query=""
// - "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi/"
//   -> path="/", query=""
func extractPathFromRequestURI() (path string, query string) {
	uri := os.Getenv("REQUEST_URI")

	// Find the go-index.cgi marker (Requirement 3.3)
	idx := strings.Index(uri, cgiMarker)
	if idx == -1 {
		// Marker not found, return default path (Requirement 3.4)
		return "/", ""
	}

	// Extract the path after the marker
	relPath := uri[idx+len(cgiMarker):]

	// Handle empty path - default to "/" (Requirement 3.4)
	if relPath == "" {
		return "/", ""
	}

	// Separate path and query string (Requirement 3.5)
	if qIdx := strings.Index(relPath, "?"); qIdx != -1 {
		path = relPath[:qIdx]
		query = relPath[qIdx+1:]
	} else {
		path = relPath
		query = ""
	}

	// Ensure path starts with "/" and handle edge case of just "/"
	if path == "" || path == "/" {
		return "/", query
	}

	return path, query
}

// extractCGIPrefix extracts the CGI prefix path from REQUEST_URI
// The CGI prefix is the part up to and including "index.cgi"
// This is used to construct the DOC_SERVER_PATH for frontend access.
//
// Requirements: 3.2, 3.3
//
// Examples:
// - "/cgi/ThirdParty/onlyoffice-fnos/index.cgi/editor?path=/vol1/test.docx"
//   -> "/cgi/ThirdParty/onlyoffice-fnos/index.cgi"
// - "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi/doc-svr/api.js"
//   -> "/cgi/ThirdParty/onlyoffice-fnos/go-index.cgi"
// - "/some/other/path" (no marker)
//   -> ""
func extractCGIPrefix() string {
	uri := os.Getenv("REQUEST_URI")

	// Remove query string if present
	if qIdx := strings.Index(uri, "?"); qIdx != -1 {
		uri = uri[:qIdx]
	}

	// Try to find go-index.cgi first (primary marker)
	if idx := strings.Index(uri, cgiMarker); idx != -1 {
		return uri[:idx+len(cgiMarker)]
	}

	// Try to find index.cgi (alternative marker)
	if idx := strings.Index(uri, indexCGIMarker); idx != -1 {
		return uri[:idx+len(indexCGIMarker)]
	}

	// No CGI marker found
	return ""
}

// buildDocServerPath constructs the full Document Server path for frontend access
// It combines the HTTP host with the CGI prefix and "/doc-svr" suffix.
//
// Requirements: 3.2, 3.3
//
// Format: <http_host><cgi_prefix>/doc-svr
// Examples:
// - httpHost="192.168.1.177:5666", cgiPrefix="/cgi/ThirdParty/onlyoffice-fnos/index.cgi"
//   -> "192.168.1.177:5666/cgi/ThirdParty/onlyoffice-fnos/index.cgi/doc-svr"
// - httpHost="example.com", cgiPrefix="/cgi/ThirdParty/onlyoffice-fnos/index.cgi"
//   -> "example.com/cgi/ThirdParty/onlyoffice-fnos/index.cgi/doc-svr"
//
// Returns empty string if httpHost or cgiPrefix is empty.
func buildDocServerPath(httpHost, cgiPrefix string) string {
	if httpHost == "" || cgiPrefix == "" {
		return ""
	}

	return httpHost + cgiPrefix + "/doc-svr"
}

// getHTTPHost returns the HTTP host from CGI environment variables
// It checks HTTP_HOST first, then falls back to SERVER_NAME:SERVER_PORT
func getHTTPHost() string {
	// Try HTTP_HOST first (includes port if non-standard)
	if host := os.Getenv("HTTP_HOST"); host != "" {
		return host
	}

	// Fall back to SERVER_NAME and SERVER_PORT
	serverName := os.Getenv("SERVER_NAME")
	if serverName == "" {
		return ""
	}

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort != "" && serverPort != "80" && serverPort != "443" {
		return serverName + ":" + serverPort
	}

	return serverName
}
