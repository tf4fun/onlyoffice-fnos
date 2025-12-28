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

// Settings represents the application configuration
type Settings struct {
	DocumentServerURL    string `json:"documentServerUrl"`
	DocumentServerSecret string `json:"documentServerSecret"`
	BaseURL              string `json:"baseUrl"` // Base URL for callbacks (e.g., http://192.168.1.100:10099)
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

// Load reads settings from the JSON file
func (s *SettingsStore) Load() (*Settings, error) {
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
