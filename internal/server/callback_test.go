package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"pgregory.net/rapid"

	"onlyoffice-fnos/internal/config"
	"onlyoffice-fnos/internal/file"
	"onlyoffice-fnos/internal/format"
	"onlyoffice-fnos/internal/jwt"
)

// Property 3: 文档保存完整性
// *For any* 状态为 2 的有效回调，下载内容应与保存内容一致
// **Validates: Requirements 2.2, 2.3**
func TestProperty3_DocumentSaveIntegrity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random file content
		contentSize := rapid.IntRange(1, 10000).Draw(t, "contentSize")
		content := make([]byte, contentSize)
		for i := range content {
			content[i] = byte(rapid.IntRange(0, 255).Draw(t, "byte"))
		}

		// Create temp directory for test
		tempDir, err := os.MkdirTemp("", "callback_test_*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Generate random filename
		filename := rapid.StringMatching(`[a-z]{3,10}\.docx`).Draw(t, "filename")
		filePath := filepath.Join(tempDir, filename)

		// Create a mock server that serves the document content
		mockDocServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(content)
		}))
		defer mockDocServer.Close()

		// Setup server components
		settingsPath := filepath.Join(tempDir, "config.json")
		settingsStore := config.NewSettingsStore(settingsPath)
		fileService := file.NewService(tempDir, 0)
		formatManager := format.NewManager()
		jwtManager := jwt.NewManager()

		// Save settings (no JWT secret for simplicity)
		settings := &config.Settings{
			DocumentServerURL: mockDocServer.URL,
		}
		if err := settingsStore.Save(settings); err != nil {
			t.Fatalf("Failed to save settings: %v", err)
		}

		// Create server
		server := New(&Config{
			SettingsStore: settingsStore,
			FileService:   fileService,
			FormatManager: formatManager,
			JWTManager:    jwtManager,
			BaseURL:       "http://localhost:8080",
		})

		// Create callback request with status 2 (saved)
		callbackReq := CallbackRequest{
			Key:    "test-key-123",
			Status: StatusSaved,
			URL:    mockDocServer.URL + "/document",
		}
		reqBody, _ := json.Marshal(callbackReq)

		// Send callback request
		req := httptest.NewRequest("POST", "/callback?path="+filename, bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		// Check response
		if rec.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", rec.Code)
		}

		var resp CallbackResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Error != 0 {
			t.Fatalf("Expected error 0, got %d", resp.Error)
		}

		// Verify saved content matches original content
		savedContent, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read saved file: %v", err)
		}

		if !bytes.Equal(savedContent, content) {
			t.Fatalf("Saved content does not match original content. Original: %d bytes, Saved: %d bytes",
				len(content), len(savedContent))
		}
	})
}

// Test callback with missing path
func TestCallbackMissingPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "callback_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	server := createTestServer(t, tempDir)

	callbackReq := CallbackRequest{
		Key:    "test-key",
		Status: StatusSaved,
		URL:    "http://example.com/doc",
	}
	reqBody, _ := json.Marshal(callbackReq)

	req := httptest.NewRequest("POST", "/callback", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	var resp CallbackResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Error != 1 {
		t.Fatalf("Expected error 1 for missing path, got %d", resp.Error)
	}
}

// Test callback with JWT verification
func TestCallbackWithJWTVerification(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "callback_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup with JWT secret
	settingsPath := filepath.Join(tempDir, "config.json")
	settingsStore := config.NewSettingsStore(settingsPath)
	jwtManager := jwt.NewManager()
	secret := jwtManager.GenerateSecret()

	settings := &config.Settings{
		DocumentServerURL:    "http://example.com",
		DocumentServerSecret: secret,
	}
	settingsStore.Save(settings)

	server := New(&Config{
		SettingsStore: settingsStore,
		FileService:   file.NewService(tempDir, 0),
		FormatManager: format.NewManager(),
		JWTManager:    jwtManager,
		BaseURL:       "http://localhost:8080",
	})

	// Test with missing token
	callbackReq := CallbackRequest{
		Key:    "test-key",
		Status: StatusEditing,
	}
	reqBody, _ := json.Marshal(callbackReq)

	req := httptest.NewRequest("POST", "/callback?path=test.docx", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	var resp CallbackResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Error != 1 {
		t.Fatalf("Expected error 1 for missing JWT token, got %d", resp.Error)
	}

	// Test with valid token
	token, _ := jwtManager.Sign(secret, map[string]interface{}{
		"key":    "test-key",
		"status": StatusEditing,
	})

	callbackReq.Token = token
	reqBody, _ = json.Marshal(callbackReq)

	req = httptest.NewRequest("POST", "/callback?path=test.docx", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Error != 0 {
		t.Fatalf("Expected error 0 for valid JWT token, got %d", resp.Error)
	}
}

// Test callback status handling
func TestCallbackStatusHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "callback_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	server := createTestServer(t, tempDir)

	testCases := []struct {
		name   string
		status CallbackStatus
		url    string
		error  int
	}{
		{"Editing", StatusEditing, "", 0},
		{"Closed", StatusClosed, "", 0},
		{"SaveError", StatusSaveError, "", 0},
		{"SavedWithoutURL", StatusSaved, "", 1}, // Should fail without URL
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			callbackReq := CallbackRequest{
				Key:    "test-key",
				Status: tc.status,
				URL:    tc.url,
			}
			reqBody, _ := json.Marshal(callbackReq)

			req := httptest.NewRequest("POST", "/callback?path=test.docx", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			server.ServeHTTP(rec, req)

			var resp CallbackResponse
			json.NewDecoder(rec.Body).Decode(&resp)

			if resp.Error != tc.error {
				t.Fatalf("Expected error %d, got %d", tc.error, resp.Error)
			}
		})
	}
}

// Helper function to create a test server
func createTestServer(t *testing.T, tempDir string) *Server {
	settingsPath := filepath.Join(tempDir, "config.json")
	settingsStore := config.NewSettingsStore(settingsPath)

	// Save default settings
	settings := &config.Settings{
		DocumentServerURL: "http://example.com",
	}
	settingsStore.Save(settings)

	return New(&Config{
		SettingsStore: settingsStore,
		FileService:   file.NewService(tempDir, 0),
		FormatManager: format.NewManager(),
		JWTManager:    jwt.NewManager(),
		BaseURL:       "http://localhost:8080",
	})
}

// Helper to read all from ReadCloser
func readAll(rc io.ReadCloser) ([]byte, error) {
	defer rc.Close()
	return io.ReadAll(rc)
}
