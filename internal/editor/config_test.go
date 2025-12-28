package editor

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"onlyoffice-fnos/internal/file"
	"onlyoffice-fnos/internal/format"
	"onlyoffice-fnos/internal/jwt"

	"pgregory.net/rapid"
)

// Property 1: 编辑器配置包含有效下载 URL
// *For any* 有效文件路径，配置中的 URL 应指向该文件
// **Validates: Requirements 1.2, 1.3**
func TestProperty1_EditorConfigContainsValidDownloadURL(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		formatManager := format.NewManager()
		jwtManager := jwt.NewManager()
		builder := NewConfigBuilder(formatManager, jwtManager)

		// Generate random file path with supported extension
		extensions := []string{"docx", "xlsx", "pptx", "doc", "xls", "ppt", "pdf"}
		extIdx := rapid.IntRange(0, len(extensions)-1).Draw(t, "extIndex")
		ext := extensions[extIdx]

		// Generate random path components
		dirParts := rapid.IntRange(1, 3).Draw(t, "dirDepth")
		pathParts := make([]string, dirParts+1)
		for i := 0; i < dirParts; i++ {
			pathParts[i] = rapid.StringMatching(`[a-zA-Z0-9_-]{1,20}`).Draw(t, "dirPart")
		}
		fileName := rapid.StringMatching(`[a-zA-Z0-9_-]{1,20}`).Draw(t, "fileName")
		pathParts[dirParts] = fileName + "." + ext
		filePath := strings.Join(pathParts, "/")

		baseURL := "http://localhost:8080"

		fileInfo := &file.FileInfo{
			Path:      filePath,
			Name:      fileName + "." + ext,
			Extension: ext,
			Size:      1024,
			ModTime:   time.Now(),
		}

		req := &ConfigRequest{
			FilePath: filePath,
			FileInfo: fileInfo,
			UserID:   "user1",
			UserName: "Test User",
			Lang:     "en",
			BaseURL:  baseURL,
		}

		config, err := builder.BuildConfig(req)
		if err != nil {
			t.Fatalf("failed to build config: %v", err)
		}

		// Verify URL is valid
		parsedURL, err := url.Parse(config.Document.URL)
		if err != nil {
			t.Fatalf("document URL is not valid: %v", err)
		}

		// Verify URL contains the file path
		queryPath := parsedURL.Query().Get("path")
		if queryPath != filePath {
			t.Fatalf("URL path parameter mismatch: expected %q, got %q", filePath, queryPath)
		}

		// Verify URL points to download endpoint
		if !strings.Contains(config.Document.URL, "/download") {
			t.Fatalf("URL should point to download endpoint, got: %s", config.Document.URL)
		}

		// Verify URL starts with base URL
		if !strings.HasPrefix(config.Document.URL, baseURL) {
			t.Fatalf("URL should start with base URL %s, got: %s", baseURL, config.Document.URL)
		}
	})
}

// Property 5: 文档密钥唯一性
// *For any* 两个不同会话，文档密钥应不同
// **Validates: Requirements 5.1**
func TestProperty5_DocumentKeyUniqueness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		formatManager := format.NewManager()
		jwtManager := jwt.NewManager()
		builder := NewConfigBuilder(formatManager, jwtManager)

		// Generate two different scenarios
		scenario := rapid.IntRange(0, 2).Draw(t, "scenario")

		var filePath1, filePath2 string
		var modTime1, modTime2 time.Time

		switch scenario {
		case 0:
			// Different files
			filePath1 = rapid.StringMatching(`[a-zA-Z0-9/]{5,30}\.docx`).Draw(t, "path1")
			filePath2 = rapid.StringMatching(`[a-zA-Z0-9/]{5,30}\.docx`).Draw(t, "path2")
			// Ensure paths are different
			if filePath1 == filePath2 {
				filePath2 = filePath2 + "_different"
			}
			modTime1 = time.Now()
			modTime2 = time.Now()
		case 1:
			// Same file, different modification times
			filePath1 = rapid.StringMatching(`[a-zA-Z0-9/]{5,30}\.docx`).Draw(t, "path")
			filePath2 = filePath1
			modTime1 = time.Now()
			modTime2 = modTime1.Add(time.Second) // Different mod time
		case 2:
			// Different files with same name in different directories
			fileName := rapid.StringMatching(`[a-zA-Z0-9]{5,15}\.docx`).Draw(t, "fileName")
			dir1 := rapid.StringMatching(`[a-zA-Z0-9]{3,10}`).Draw(t, "dir1")
			dir2 := rapid.StringMatching(`[a-zA-Z0-9]{3,10}`).Draw(t, "dir2")
			if dir1 == dir2 {
				dir2 = dir2 + "_different"
			}
			filePath1 = dir1 + "/" + fileName
			filePath2 = dir2 + "/" + fileName
			modTime1 = time.Now()
			modTime2 = time.Now()
		}

		key1 := builder.GetDocumentKey(filePath1, modTime1)
		key2 := builder.GetDocumentKey(filePath2, modTime2)

		// Keys should be different for different sessions
		if key1 == key2 {
			t.Fatalf("document keys should be different for different sessions: path1=%s, path2=%s, modTime1=%v, modTime2=%v, key=%s",
				filePath1, filePath2, modTime1, modTime2, key1)
		}

		// Keys should have consistent length
		if len(key1) != 20 || len(key2) != 20 {
			t.Fatalf("document keys should be 20 characters: len(key1)=%d, len(key2)=%d", len(key1), len(key2))
		}
	})
}

// Property 6: 编辑器配置包含用户信息
// *For any* 配置，user 字段应包含非空 id 和 name
// **Validates: Requirements 5.2**
func TestProperty6_EditorConfigContainsUserInfo(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		formatManager := format.NewManager()
		jwtManager := jwt.NewManager()
		builder := NewConfigBuilder(formatManager, jwtManager)

		// Generate random user info (including empty values to test defaults)
		userID := rapid.OneOf(
			rapid.Just(""),
			rapid.StringMatching(`[a-zA-Z0-9]{1,20}`),
		).Draw(t, "userID")

		userName := rapid.OneOf(
			rapid.Just(""),
			rapid.StringMatching(`[a-zA-Z0-9 ]{1,30}`),
		).Draw(t, "userName")

		fileInfo := &file.FileInfo{
			Path:      "/test/document.docx",
			Name:      "document.docx",
			Extension: "docx",
			Size:      1024,
			ModTime:   time.Now(),
		}

		req := &ConfigRequest{
			FilePath: "/test/document.docx",
			FileInfo: fileInfo,
			UserID:   userID,
			UserName: userName,
			Lang:     "en",
			BaseURL:  "http://localhost:8080",
		}

		config, err := builder.BuildConfig(req)
		if err != nil {
			t.Fatalf("failed to build config: %v", err)
		}

		// Verify user ID is non-empty
		if config.EditorConfig.User.ID == "" {
			t.Fatal("user ID should not be empty")
		}

		// Verify user name is non-empty
		if config.EditorConfig.User.Name == "" {
			t.Fatal("user name should not be empty")
		}
	})
}

// Property 8: 编辑器配置包含语言设置
// *For any* 配置，lang 字段应包含有效语言代码
// **Validates: Requirements 6.2**
func TestProperty8_EditorConfigContainsLanguageSetting(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		formatManager := format.NewManager()
		jwtManager := jwt.NewManager()
		builder := NewConfigBuilder(formatManager, jwtManager)

		// Generate random language input (including empty and various formats)
		langInput := rapid.OneOf(
			rapid.Just(""),
			rapid.Just("en"),
			rapid.Just("zh"),
			rapid.Just("zh-CN"),
			rapid.Just("zh-TW"),
			rapid.Just("en-US"),
			rapid.Just("EN"),
			rapid.Just("ZH-CN"),
			rapid.StringMatching(`[a-zA-Z]{2}`),
			rapid.StringMatching(`[a-zA-Z]{2}-[a-zA-Z]{2}`),
		).Draw(t, "lang")

		fileInfo := &file.FileInfo{
			Path:      "/test/document.docx",
			Name:      "document.docx",
			Extension: "docx",
			Size:      1024,
			ModTime:   time.Now(),
		}

		req := &ConfigRequest{
			FilePath: "/test/document.docx",
			FileInfo: fileInfo,
			UserID:   "user1",
			UserName: "Test User",
			Lang:     langInput,
			BaseURL:  "http://localhost:8080",
		}

		config, err := builder.BuildConfig(req)
		if err != nil {
			t.Fatalf("failed to build config: %v", err)
		}

		// Verify lang is non-empty
		if config.EditorConfig.Lang == "" {
			t.Fatal("lang should not be empty")
		}

		// Verify lang is a valid 2-character code
		if len(config.EditorConfig.Lang) != 2 {
			t.Fatalf("lang should be a 2-character code, got: %s (len=%d)", config.EditorConfig.Lang, len(config.EditorConfig.Lang))
		}

		// Verify lang is lowercase
		if config.EditorConfig.Lang != strings.ToLower(config.EditorConfig.Lang) {
			t.Fatalf("lang should be lowercase, got: %s", config.EditorConfig.Lang)
		}
	})
}

// Unit test: BuildConfig with nil request
func TestBuildConfigNilRequest(t *testing.T) {
	formatManager := format.NewManager()
	jwtManager := jwt.NewManager()
	builder := NewConfigBuilder(formatManager, jwtManager)

	_, err := builder.BuildConfig(nil)
	if err == nil {
		t.Error("BuildConfig should return error for nil request")
	}
}

// Unit test: BuildConfig with nil FileInfo
func TestBuildConfigNilFileInfo(t *testing.T) {
	formatManager := format.NewManager()
	jwtManager := jwt.NewManager()
	builder := NewConfigBuilder(formatManager, jwtManager)

	req := &ConfigRequest{
		FilePath: "/test/document.docx",
		FileInfo: nil,
	}

	_, err := builder.BuildConfig(req)
	if err == nil {
		t.Error("BuildConfig should return error for nil FileInfo")
	}
}

// Unit test: BuildConfig with unsupported format
func TestBuildConfigUnsupportedFormat(t *testing.T) {
	formatManager := format.NewManager()
	jwtManager := jwt.NewManager()
	builder := NewConfigBuilder(formatManager, jwtManager)

	fileInfo := &file.FileInfo{
		Path:      "/test/document.xyz",
		Name:      "document.xyz",
		Extension: "xyz",
		Size:      1024,
		ModTime:   time.Now(),
	}

	req := &ConfigRequest{
		FilePath: "/test/document.xyz",
		FileInfo: fileInfo,
		UserID:   "user1",
		UserName: "Test User",
		Lang:     "en",
		BaseURL:  "http://localhost:8080",
	}

	_, err := builder.BuildConfig(req)
	if err == nil {
		t.Error("BuildConfig should return error for unsupported format")
	}
}

// Unit test: BuildConfig sets correct edit mode for editable formats
func TestBuildConfigEditMode(t *testing.T) {
	formatManager := format.NewManager()
	jwtManager := jwt.NewManager()
	builder := NewConfigBuilder(formatManager, jwtManager)

	tests := []struct {
		ext          string
		expectedMode string
		canEdit      bool
	}{
		{"docx", "edit", true},
		{"xlsx", "edit", true},
		{"pptx", "edit", true},
		{"pdf", "view", false},
		{"doc", "view", false},
	}

	for _, tt := range tests {
		fileInfo := &file.FileInfo{
			Path:      "/test/document." + tt.ext,
			Name:      "document." + tt.ext,
			Extension: tt.ext,
			Size:      1024,
			ModTime:   time.Now(),
		}

		req := &ConfigRequest{
			FilePath: "/test/document." + tt.ext,
			FileInfo: fileInfo,
			UserID:   "user1",
			UserName: "Test User",
			Lang:     "en",
			BaseURL:  "http://localhost:8080",
		}

		config, err := builder.BuildConfig(req)
		if err != nil {
			t.Errorf("failed to build config for %s: %v", tt.ext, err)
			continue
		}

		if config.EditorConfig.Mode != tt.expectedMode {
			t.Errorf("expected mode %s for %s, got %s", tt.expectedMode, tt.ext, config.EditorConfig.Mode)
		}

		if config.Document.Permissions.Edit != tt.canEdit {
			t.Errorf("expected edit permission %v for %s, got %v", tt.canEdit, tt.ext, config.Document.Permissions.Edit)
		}
	}
}

// Unit test: BuildConfig with JWT signing
func TestBuildConfigWithJWT(t *testing.T) {
	formatManager := format.NewManager()
	jwtManager := jwt.NewManager()
	builder := NewConfigBuilder(formatManager, jwtManager)

	secret := jwtManager.GenerateSecret()

	fileInfo := &file.FileInfo{
		Path:      "/test/document.docx",
		Name:      "document.docx",
		Extension: "docx",
		Size:      1024,
		ModTime:   time.Now(),
	}

	req := &ConfigRequest{
		FilePath:  "/test/document.docx",
		FileInfo:  fileInfo,
		UserID:    "user1",
		UserName:  "Test User",
		Lang:      "en",
		BaseURL:   "http://localhost:8080",
		JWTSecret: secret,
	}

	config, err := builder.BuildConfig(req)
	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	// Verify token is present
	if config.Token == "" {
		t.Error("token should be present when JWTSecret is provided")
	}

	// Verify token is valid
	_, err = jwtManager.Verify(secret, config.Token)
	if err != nil {
		t.Errorf("token should be valid: %v", err)
	}
}

// Unit test: Document key consistency
func TestDocumentKeyConsistency(t *testing.T) {
	formatManager := format.NewManager()
	jwtManager := jwt.NewManager()
	builder := NewConfigBuilder(formatManager, jwtManager)

	filePath := "/test/document.docx"
	modTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// Same inputs should produce same key
	key1 := builder.GetDocumentKey(filePath, modTime)
	key2 := builder.GetDocumentKey(filePath, modTime)

	if key1 != key2 {
		t.Errorf("same inputs should produce same key: %s != %s", key1, key2)
	}
}
