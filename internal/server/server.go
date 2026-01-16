package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"onlyoffice-fnos/internal/config"
	"onlyoffice-fnos/internal/editor"
	"onlyoffice-fnos/internal/file"
	"onlyoffice-fnos/internal/format"
	"onlyoffice-fnos/internal/jwt"
	"onlyoffice-fnos/web"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

// OriginalRemoteAddrKey is the context key for storing the original RemoteAddr
const OriginalRemoteAddrKey contextKey = "originalRemoteAddr"

// Server represents the HTTP server
type Server struct {
	router        *chi.Mux
	settings      *config.Settings
	fileService   *file.Service
	formatManager *format.Manager
	jwtManager    *jwt.Manager
	configBuilder *editor.ConfigBuilder
	baseURL       string
	templates     *templates
}

// Config holds server configuration
type Config struct {
	Settings      *config.Settings
	FileService   *file.Service
	FormatManager *format.Manager
	JWTManager    *jwt.Manager
	BaseURL       string
}

// New creates a new Server instance
func New(cfg *Config) *Server {
	s := &Server{
		router:        chi.NewRouter(),
		settings:      cfg.Settings,
		fileService:   cfg.FileService,
		formatManager: cfg.FormatManager,
		jwtManager:    cfg.JWTManager,
		baseURL:       cfg.BaseURL,
	}

	// Use baseURL from settings if available
	if cfg.Settings != nil && cfg.Settings.BaseURL != "" {
		s.baseURL = cfg.Settings.BaseURL
	}
	// If still empty after loading settings, use the provided config (command line default)
	if s.baseURL == "" && cfg.BaseURL != "" {
		s.baseURL = cfg.BaseURL
	}

	// Create config builder
	s.configBuilder = editor.NewConfigBuilder(cfg.FormatManager, cfg.JWTManager)

	// Load embedded templates
	if err := s.loadTemplates(); err != nil {
		log.Printf("Warning: failed to load templates: %v", err)
	}

	// Setup middleware
	// IMPORTANT: CaptureOriginalRemoteAddr must be BEFORE RealIP
	// so we can capture the original RemoteAddr for the proxy route
	s.router.Use(CaptureOriginalRemoteAddr)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Timeout(60 * time.Second))

	// Setup routes
	s.setupRoutes()

	return s
}

// CaptureOriginalRemoteAddr is a middleware that captures the original RemoteAddr
// before the RealIP middleware modifies it. This is needed for the reverse proxy
// to correctly set X-Forwarded-For headers.
func CaptureOriginalRemoteAddr(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), OriginalRemoteAddrKey, r.RemoteAddr)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetOriginalRemoteAddr retrieves the original RemoteAddr from the request context.
// If not found, it returns the current RemoteAddr.
func GetOriginalRemoteAddr(r *http.Request) string {
	if addr, ok := r.Context().Value(OriginalRemoteAddrKey).(string); ok {
		return addr
	}
	return r.RemoteAddr
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Embedded static files
	staticFS, err := fs.Sub(web.Static, "static")
	if err != nil {
		log.Printf("Warning: failed to get static sub-filesystem: %v", err)
	} else {
		s.router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	}

	// Page routes
	s.router.Get("/editor", s.handleEditorPage)
	s.router.Get("/convert", s.handleConvertPage)

	// Document Server integration routes
	s.router.Get("/download", s.handleDownload)
	s.router.Post("/callback", s.handleCallback)
	s.router.Post("/convert", s.handleConvert)

	// Document Server reverse proxy route
	// Proxies requests from /doc-svr/* to the configured Document Server URL
	s.router.Handle("/doc-svr/*", http.HandlerFunc(s.handleDocServerProxy))
}

// Router returns the chi router for testing
func (s *Server) Router() *chi.Mux {
	return s.router
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Start starts the HTTP server
func (s *Server) Start(addr string) error {
	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	// Chi router doesn't have built-in shutdown, but we can use http.Server
	return nil
}

// JSON response helpers
func (s *Server) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func (s *Server) respondError(w http.ResponseWriter, status int, message string) {
	s.respondJSON(w, status, map[string]interface{}{
		"error":   status,
		"message": message,
	})
}
