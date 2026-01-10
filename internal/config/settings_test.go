package config

import (
	"os"
	"testing"
)

// Unit test: LoadFromEnv returns error when no env vars are set
func TestLoadFromEnvNoVars(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv(EnvDocumentServerURL)
	os.Unsetenv(EnvDocumentServerSecret)
	os.Unsetenv(EnvBaseURL)

	_, err := LoadFromEnv()
	if err != ErrConfigNotFound {
		t.Errorf("expected ErrConfigNotFound, got %v", err)
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

	settings, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

// Unit test: LoadFromEnv returns settings when only some env vars are set
func TestLoadFromEnvPartialVars(t *testing.T) {
	os.Unsetenv(EnvDocumentServerURL)
	os.Unsetenv(EnvDocumentServerSecret)
	os.Unsetenv(EnvBaseURL)

	// Set only one env var
	os.Setenv(EnvDocumentServerURL, "http://docs.example.com")
	defer os.Unsetenv(EnvDocumentServerURL)

	settings, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if settings.DocumentServerURL != "http://docs.example.com" {
		t.Errorf("DocumentServerURL mismatch: expected %q, got %q",
			"http://docs.example.com", settings.DocumentServerURL)
	}
	if settings.DocumentServerSecret != "" {
		t.Errorf("DocumentServerSecret should be empty, got %q", settings.DocumentServerSecret)
	}
	if settings.BaseURL != "" {
		t.Errorf("BaseURL should be empty, got %q", settings.BaseURL)
	}
}
