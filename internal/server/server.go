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

// Server represents the HTTP server
type Server struct {
	router        *chi.Mux
	settingsStore *config.SettingsStore
	fileService   *file.Service
	formatManager *format.Manager
	jwtManager    *jwt.Manager
	configBuilder *editor.ConfigBuilder
	baseURL       string
	templates     *templates
}

// Config holds server configuration
type Config struct {
	SettingsStore *config.SettingsStore
	FileService   *file.Service
	FormatManager *format.Manager
	JWTManager    *jwt.Manager
	BaseURL       string
}

// New creates a new Server instance
func New(cfg *Config) *Server {
	s := &Server{
		router:        chi.NewRouter(),
		settingsStore: cfg.SettingsStore,
		fileService:   cfg.FileService,
		formatManager: cfg.FormatManager,
		jwtManager:    cfg.JWTManager,
		baseURL:       cfg.BaseURL,
	}

	// Create config builder
	s.configBuilder = editor.NewConfigBuilder(cfg.FormatManager, cfg.JWTManager)

	// Load embedded templates
	if err := s.loadTemplates(); err != nil {
		log.Printf("Warning: failed to load templates: %v", err)
	}

	// Setup middleware
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Timeout(60 * time.Second))

	// Setup routes
	s.setupRoutes()

	return s
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
	s.router.Get("/", s.handleSettingsPage)
	s.router.Get("/editor", s.handleEditorPage)
	s.router.Get("/convert", s.handleConvertPage)

	// API routes
	s.router.Route("/api", func(r chi.Router) {
		r.Get("/settings", s.handleGetSettings)
		r.Post("/settings", s.handleSaveSettings)
		r.Post("/settings/generate-key", s.handleGenerateKey)
		r.Post("/settings/validate", s.handleValidateConnection)
	})

	// Document Server integration routes
	s.router.Get("/download", s.handleDownload)
	s.router.Post("/callback", s.handleCallback)
	s.router.Post("/convert", s.handleConvert)
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
