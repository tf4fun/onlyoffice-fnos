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
		DocumentServerURL:    "http://localhost:8080",
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
