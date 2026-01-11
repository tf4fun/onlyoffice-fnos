package config

import (
	"errors"
	"os"
)

var (
	ErrConfigNotFound = errors.New("configuration not found")
)

// Environment variable names
const (
	EnvDocumentServerURL    = "DOCUMENT_SERVER_URL"
	EnvDocumentServerPubURL = "DOCUMENT_SERVER_PUB_URL"
	EnvDocumentServerSecret = "DOCUMENT_SERVER_SECRET"
	EnvBaseURL              = "BASE_URL"
	EnvDocServerPath        = "DOC_SERVER_PATH"
)

// Settings represents the application configuration
type Settings struct {
	DocumentServerURL    string `json:"documentServerUrl"`    // Internal URL for backend API calls to Document Server
	DocumentServerPubURL string `json:"documentServerPubUrl"` // Public/WAN URL for Document Server (optional, deprecated)
	DocumentServerSecret string `json:"documentServerSecret"`
	BaseURL              string `json:"baseUrl"`
	DocServerPath        string `json:"docServerPath"` // Frontend path prefix for Document Server (e.g., "/doc-svr")
}

// LoadFromEnv loads settings from environment variables.
// Returns ErrConfigNotFound if no environment variables are set.
func LoadFromEnv() (*Settings, error) {
	url := os.Getenv(EnvDocumentServerURL)
	pubURL := os.Getenv(EnvDocumentServerPubURL)
	secret := os.Getenv(EnvDocumentServerSecret)
	baseURL := os.Getenv(EnvBaseURL)
	docServerPath := os.Getenv(EnvDocServerPath)

	// Return error if no env vars are set
	if url == "" && pubURL == "" && secret == "" && baseURL == "" && docServerPath == "" {
		return nil, ErrConfigNotFound
	}

	return &Settings{
		DocumentServerURL:    url,
		DocumentServerPubURL: pubURL,
		DocumentServerSecret: secret,
		BaseURL:              baseURL,
		DocServerPath:        docServerPath,
	}, nil
}
