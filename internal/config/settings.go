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
	EnvDocumentServerSecret = "DOCUMENT_SERVER_SECRET"
	EnvBaseURL              = "BASE_URL"
)

// Settings represents the application configuration
type Settings struct {
	DocumentServerURL    string `json:"documentServerUrl"`
	DocumentServerSecret string `json:"documentServerSecret"`
	BaseURL              string `json:"baseUrl"`
}

// LoadFromEnv loads settings from environment variables.
// Returns ErrConfigNotFound if no environment variables are set.
func LoadFromEnv() (*Settings, error) {
	url := os.Getenv(EnvDocumentServerURL)
	secret := os.Getenv(EnvDocumentServerSecret)
	baseURL := os.Getenv(EnvBaseURL)

	// Return error if no env vars are set
	if url == "" && secret == "" && baseURL == "" {
		return nil, ErrConfigNotFound
	}

	return &Settings{
		DocumentServerURL:    url,
		DocumentServerSecret: secret,
		BaseURL:              baseURL,
	}, nil
}
