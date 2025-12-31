package editor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"onlyoffice-fnos/internal/file"
	"onlyoffice-fnos/internal/format"
	"onlyoffice-fnos/internal/jwt"
)

// EditorConfig represents the complete OnlyOffice editor configuration
type EditorConfig struct {
	Document     DocumentConfig     `json:"document"`
	DocumentType string             `json:"documentType"` // word, cell, slide
	EditorConfig EditorConfigInner  `json:"editorConfig"`
	Token        string             `json:"token,omitempty"`
}

// DocumentConfig represents the document configuration
type DocumentConfig struct {
	FileType    string            `json:"fileType"`
	Key         string            `json:"key"`
	Title       string            `json:"title"`
	URL         string            `json:"url"`
	Permissions PermissionsConfig `json:"permissions"`
}

// PermissionsConfig represents document permissions
type PermissionsConfig struct {
	Edit     bool `json:"edit"`
	Download bool `json:"download"`
	Print    bool `json:"print"`
}

// EditorConfigInner represents the inner editor configuration
type EditorConfigInner struct {
	CallbackURL string     `json:"callbackUrl"`
	Lang        string     `json:"lang"`
	Mode        string     `json:"mode"` // edit, view
	User        UserConfig `json:"user"`
}

// UserConfig represents user information
type UserConfig struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ConfigRequest represents a request to build editor configuration
type ConfigRequest struct {
	FilePath    string
	FileInfo    *file.FileInfo
	UserID      string
	UserName    string
	Lang        string
	BaseURL     string // Base URL for download and callback endpoints
	JWTSecret   string
}

// ConfigBuilder builds OnlyOffice editor configurations
type ConfigBuilder struct {
	formatManager *format.Manager
	jwtManager    *jwt.Manager
}

// NewConfigBuilder creates a new ConfigBuilder
func NewConfigBuilder(formatManager *format.Manager, jwtManager *jwt.Manager) *ConfigBuilder {
	return &ConfigBuilder{
		formatManager: formatManager,
		jwtManager:    jwtManager,
	}
}

// BuildConfig builds an OnlyOffice editor configuration
func (b *ConfigBuilder) BuildConfig(req *ConfigRequest) (*EditorConfig, error) {
	if req == nil || req.FileInfo == nil {
		return nil, fmt.Errorf("invalid config request")
	}

	// Get format information
	formatInfo, ok := b.formatManager.GetFormat(req.FileInfo.Extension)
	if !ok {
		return nil, fmt.Errorf("unsupported file format: %s", req.FileInfo.Extension)
	}

	// Determine edit mode
	canEdit := formatInfo.Editable
	mode := "view"
	if canEdit {
		mode = "edit"
	}

	// Generate document key
	docKey := b.generateDocumentKey(req.FilePath, req.FileInfo.ModTime)

	// Build download URL
	downloadURL := b.buildDownloadURL(req.BaseURL, req.FilePath)

	// Build callback URL
	callbackURL := b.buildCallbackURL(req.BaseURL, req.FilePath)

	// Normalize language code
	lang := b.normalizeLanguage(req.Lang)

	// Ensure user info is not empty
	userID := req.UserID
	if userID == "" {
		userID = "anonymous"
	}
	userName := req.UserName
	if userName == "" {
		userName = "Anonymous User"
	}

	config := &EditorConfig{
		Document: DocumentConfig{
			FileType: req.FileInfo.Extension,
			Key:      docKey,
			Title:    req.FileInfo.Name,
			URL:      downloadURL,
			Permissions: PermissionsConfig{
				Edit:     canEdit,
				Download: true,
				Print:    true,
			},
		},
		DocumentType: formatInfo.Type,
		EditorConfig: EditorConfigInner{
			CallbackURL: callbackURL,
			Lang:        lang,
			Mode:        mode,
			User: UserConfig{
				ID:   userID,
				Name: userName,
			},
		},
	}

	// Sign the configuration with JWT if secret is provided
	if req.JWTSecret != "" {
		token, err := b.signConfig(req.JWTSecret, config)
		if err != nil {
			return nil, fmt.Errorf("failed to sign config: %w", err)
		}
		config.Token = token
	}

	return config, nil
}

// generateDocumentKey generates a unique document key based on file path and modification time
func (b *ConfigBuilder) generateDocumentKey(filePath string, modTime time.Time) string {
	// Combine file path and modification time for uniqueness
	data := fmt.Sprintf("%s|%d", filePath, modTime.UnixNano())
	hash := sha256.Sum256([]byte(data))
	// Use first 20 characters of hex-encoded hash
	return hex.EncodeToString(hash[:])[:20]
}

// buildDownloadURL builds the download URL for the document
func (b *ConfigBuilder) buildDownloadURL(baseURL, filePath string) string {
	if baseURL == "" {
		// This should not happen if properly configured
		// Log a warning in production
		baseURL = "http://localhost:10099"
	}
	// Ensure baseURL doesn't have trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")
	// URL encode the file path
	encodedPath := url.QueryEscape(filePath)
	return fmt.Sprintf("%s/download?path=%s", baseURL, encodedPath)
}

// buildCallbackURL builds the callback URL for document saving
func (b *ConfigBuilder) buildCallbackURL(baseURL, filePath string) string {
	if baseURL == "" {
		// This should not happen if properly configured
		// Log a warning in production
		baseURL = "http://localhost:10099"
	}
	// Ensure baseURL doesn't have trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")
	// URL encode the file path
	encodedPath := url.QueryEscape(filePath)
	return fmt.Sprintf("%s/callback?path=%s", baseURL, encodedPath)
}

// normalizeLanguage normalizes the language code
func (b *ConfigBuilder) normalizeLanguage(lang string) string {
	if lang == "" {
		return "en"
	}
	// Convert to lowercase and handle common formats
	lang = strings.ToLower(lang)
	// Handle zh-CN, zh-TW, etc.
	if strings.HasPrefix(lang, "zh") {
		return "zh"
	}
	// Return first two characters for standard language codes
	if len(lang) >= 2 {
		return lang[:2]
	}
	return lang
}

// signConfig signs the editor configuration with JWT
func (b *ConfigBuilder) signConfig(secret string, config *EditorConfig) (string, error) {
	claims := map[string]interface{}{
		"document":     config.Document,
		"documentType": config.DocumentType,
		"editorConfig": config.EditorConfig,
	}
	return b.jwtManager.Sign(secret, claims)
}

// GetDocumentKey generates a document key for a given file path and modification time
// This is exposed for testing purposes
func (b *ConfigBuilder) GetDocumentKey(filePath string, modTime time.Time) string {
	return b.generateDocumentKey(filePath, modTime)
}

// GetFileExtension extracts the file extension from a path
func GetFileExtension(path string) string {
	ext := filepath.Ext(path)
	if ext != "" {
		return strings.ToLower(ext[1:]) // Remove leading dot
	}
	return ""
}
