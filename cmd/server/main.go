package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"onlyoffice-fnos/internal/config"
	"onlyoffice-fnos/internal/file"
	"onlyoffice-fnos/internal/format"
	"onlyoffice-fnos/internal/jwt"
	"onlyoffice-fnos/internal/server"
)

const (
	defaultPort       = "10099"
	defaultConfigFile = "config.json"
	shutdownTimeout   = 10 * time.Second
)

func main() {
	// Parse command line arguments
	var (
		configPath = flag.String("config", "", "Path to configuration file (default: <data>/config.json)")
		port       = flag.String("port", defaultPort, "HTTP server port")
		dataDir    = flag.String("data", "", "Data directory for configuration and logs")
		baseURL    = flag.String("base-url", "", "Base URL for callbacks (e.g., http://192.168.1.100:10099)")
	)
	flag.Parse()

	// Determine data directory
	if *dataDir == "" {
		// Use current directory if not specified
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get current directory: %v", err)
		}
		*dataDir = cwd
	}

	// Ensure data directory exists
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Determine config file path
	if *configPath == "" {
		*configPath = filepath.Join(*dataDir, defaultConfigFile)
	}

	// Determine base URL
	if *baseURL == "" {
		*baseURL = fmt.Sprintf("http://localhost:%s", *port)
	}

	log.Printf("OnlyOffice fnOS Connector starting...")
	log.Printf("  Config file: %s", *configPath)
	log.Printf("  Data directory: %s", *dataDir)
	log.Printf("  Port: %s", *port)
	log.Printf("  Base URL: %s", *baseURL)

	// Initialize modules
	settingsStore := config.NewSettingsStore(*configPath)
	formatManager := format.NewManager()
	jwtManager := jwt.NewManager()
	fileService := file.NewService("", 0) // No base path restriction, no size limit

	// Create server configuration
	serverConfig := &server.Config{
		SettingsStore: settingsStore,
		FileService:   fileService,
		FormatManager: formatManager,
		JWTManager:    jwtManager,
		BaseURL:       *baseURL,
	}

	// Create HTTP server
	srv := server.New(serverConfig)

	// Create HTTP server with timeouts
	httpServer := &http.Server{
		Addr:         ":" + *port,
		Handler:      srv,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Channel to listen for errors from server
	serverErrors := make(chan error, 1)

	// Start server in goroutine
	go func() {
		log.Printf("Server listening on :%s", *port)
		serverErrors <- httpServer.ListenAndServe()
	}()

	// Channel to listen for interrupt signals
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

		// Create context with timeout for graceful shutdown
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
