package config

import (
	"os"
	"path/filepath"
	"testing"

	"pgregory.net/rapid"
)

// Property 9: 设置保存和加载一致性 (Round-trip)
// *For any* 有效的设置对象，保存后再加载应得到相同值
// **Validates: Requirements 4.1, 4.2**
func TestProperty9_SettingsRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random settings
		settings := &Settings{
			DocumentServerURL:    rapid.String().Draw(t, "documentServerUrl"),
			DocumentServerSecret: rapid.String().Draw(t, "documentServerSecret"),
		}

		// Create a temporary file for testing using os.CreateTemp for safe naming
		tmpFile, err := os.CreateTemp("", "test_settings_*.json")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		tmpFilePath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpFilePath)

		store := NewSettingsStore(tmpFilePath)

		// Save settings
		if err := store.Save(settings); err != nil {
			t.Fatalf("failed to save settings: %v", err)
		}

		// Load settings back
		loaded, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load settings: %v", err)
		}

		// Verify round-trip consistency
		if loaded.DocumentServerURL != settings.DocumentServerURL {
			t.Fatalf("DocumentServerURL mismatch: expected %q, got %q",
				settings.DocumentServerURL, loaded.DocumentServerURL)
		}
		if loaded.DocumentServerSecret != settings.DocumentServerSecret {
			t.Fatalf("DocumentServerSecret mismatch: expected %q, got %q",
				settings.DocumentServerSecret, loaded.DocumentServerSecret)
		}
	})
}

// Unit test: Load returns error for non-existent file
func TestLoadNonExistentFile(t *testing.T) {
	store := NewSettingsStore("/non/existent/path/config.json")
	_, err := store.Load()
	if err != ErrConfigNotFound {
		t.Errorf("expected ErrConfigNotFound, got %v", err)
	}
}

// Unit test: Save creates directory if needed
func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "test_config_dir")
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "subdir", "config.json")
	store := NewSettingsStore(tmpFile)

	settings := &Settings{
		DocumentServerURL:    "http://localhost:10099",
		DocumentServerSecret: "secret123",
	}

	if err := store.Save(settings); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("settings file was not created")
	}
}

// Unit test: Save returns error for nil settings
func TestSaveNilSettings(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test_nil_settings.json")
	defer os.Remove(tmpFile)

	store := NewSettingsStore(tmpFile)
	err := store.Save(nil)
	if err != ErrInvalidConfig {
		t.Errorf("expected ErrInvalidConfig, got %v", err)
	}
}


// Unit test: LoadFromEnv returns nil when no env vars are set
func TestLoadFromEnvNoVars(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv(EnvDocumentServerURL)
	os.Unsetenv(EnvDocumentServerSecret)
	os.Unsetenv(EnvBaseURL)

	settings := LoadFromEnv()
	if settings != nil {
		t.Error("expected nil when no env vars are set")
	}
}

// Unit test: LoadFromEnv returns settings when env vars are set
func TestLoadFromEnvWithVars(t *testing.T) {
	// Set env vars
	os.Setenv(EnvDocumentServerURL, "http://docs.example.com")
	os.Setenv(EnvDocumentServerSecret, "test-secret")
	os.Setenv(EnvBaseURL, "http://localhost:8080")
	defer func() {
		os.Unsetenv(EnvDocumentServerURL)
		os.Unsetenv(EnvDocumentServerSecret)
		os.Unsetenv(EnvBaseURL)
	}()

	settings := LoadFromEnv()
	if settings == nil {
		t.Fatal("expected settings, got nil")
	}

	if settings.DocumentServerURL != "http://docs.example.com" {
		t.Errorf("DocumentServerURL mismatch: expected %q, got %q",
			"http://docs.example.com", settings.DocumentServerURL)
	}
	if settings.DocumentServerSecret != "test-secret" {
		t.Errorf("DocumentServerSecret mismatch: expected %q, got %q",
			"test-secret", settings.DocumentServerSecret)
	}
	if settings.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL mismatch: expected %q, got %q",
			"http://localhost:8080", settings.BaseURL)
	}
}

// Unit test: Load prefers env vars over file
func TestLoadEnvPrecedence(t *testing.T) {
	// Create a temp file with different values
	tmpFile, err := os.CreateTemp("", "test_env_precedence_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFilePath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpFilePath)

	store := NewSettingsStore(tmpFilePath)

	// Save file-based settings
	fileSettings := &Settings{
		DocumentServerURL:    "http://file-url.com",
		DocumentServerSecret: "file-secret",
		BaseURL:              "http://file-base.com",
	}
	if err := store.Save(fileSettings); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}

	// Set env vars with different values
	os.Setenv(EnvDocumentServerURL, "http://env-url.com")
	os.Setenv(EnvDocumentServerSecret, "env-secret")
	os.Setenv(EnvBaseURL, "http://env-base.com")
	defer func() {
		os.Unsetenv(EnvDocumentServerURL)
		os.Unsetenv(EnvDocumentServerSecret)
		os.Unsetenv(EnvBaseURL)
	}()

	// Load should return env values
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load settings: %v", err)
	}

	if loaded.DocumentServerURL != "http://env-url.com" {
		t.Errorf("expected env URL, got %q", loaded.DocumentServerURL)
	}
	if loaded.DocumentServerSecret != "env-secret" {
		t.Errorf("expected env secret, got %q", loaded.DocumentServerSecret)
	}
	if loaded.BaseURL != "http://env-base.com" {
		t.Errorf("expected env base URL, got %q", loaded.BaseURL)
	}
}

// Unit test: LoadFromFile ignores env vars
func TestLoadFromFileIgnoresEnv(t *testing.T) {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "test_file_only_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFilePath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpFilePath)

	store := NewSettingsStore(tmpFilePath)

	// Save file-based settings
	fileSettings := &Settings{
		DocumentServerURL:    "http://file-url.com",
		DocumentServerSecret: "file-secret",
		BaseURL:              "http://file-base.com",
	}
	if err := store.Save(fileSettings); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}

	// Set env vars
	os.Setenv(EnvDocumentServerURL, "http://env-url.com")
	defer os.Unsetenv(EnvDocumentServerURL)

	// LoadFromFile should return file values
	loaded, err := store.LoadFromFile()
	if err != nil {
		t.Fatalf("failed to load settings: %v", err)
	}

	if loaded.DocumentServerURL != "http://file-url.com" {
		t.Errorf("expected file URL, got %q", loaded.DocumentServerURL)
	}
}
