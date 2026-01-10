package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrConfigNotFound = errors.New("configuration not found")
	ErrInvalidConfig  = errors.New("invalid configuration")
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
	BaseURL              string `json:"baseUrl"` // Base URL for callbacks (e.g., http://192.168.1.100:10099)
}

// LoadFromEnv loads settings from environment variables.
// Returns nil if no environment variables are set.
func LoadFromEnv() *Settings {
	url := os.Getenv(EnvDocumentServerURL)
	secret := os.Getenv(EnvDocumentServerSecret)
	baseURL := os.Getenv(EnvBaseURL)

	// Return nil if no env vars are set
	if url == "" && secret == "" && baseURL == "" {
		return nil
	}

	return &Settings{
		DocumentServerURL:    url,
		DocumentServerSecret: secret,
		BaseURL:              baseURL,
	}
}

// SettingsStore handles loading and saving settings to a JSON file
type SettingsStore struct {
	filePath string
	mu       sync.RWMutex
}

// NewSettingsStore creates a new SettingsStore with the given file path
func NewSettingsStore(filePath string) *SettingsStore {
	return &SettingsStore{
		filePath: filePath,
	}
}

// Load reads settings from environment variables first, then falls back to JSON file.
// Environment variables take precedence over file configuration.
func (s *SettingsStore) Load() (*Settings, error) {
	// First, try to load from environment variables
	if envSettings := LoadFromEnv(); envSettings != nil {
		return envSettings, nil
	}

	// Fall back to file-based configuration
	return s.LoadFromFile()
}

// LoadFromFile reads settings from the JSON file only
func (s *SettingsStore) LoadFromFile() (*Settings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, ErrInvalidConfig
	}

	return &settings, nil
}

// Save writes settings to the JSON file
func (s *SettingsStore) Save(settings *Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if settings == nil {
		return ErrInvalidConfig
	}

	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	data, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

// GetFilePath returns the path to the settings file
func (s *SettingsStore) GetFilePath() string {
	return s.filePath
}
