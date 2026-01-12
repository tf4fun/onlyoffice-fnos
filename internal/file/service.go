package file

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrFileNotFound       = errors.New("file not found")
	ErrInvalidPath        = errors.New("invalid file path")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrSaveFailed         = errors.New("failed to save file")
	ErrFileTooLarge       = errors.New("file size exceeds limit")
)

// FileInfo represents information about a file
type FileInfo struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Extension string    `json:"extension"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"modTime"`
}

// Service handles file operations for fnOS file system
type Service struct {
	// basePath is the root path for file operations (optional, for security)
	basePath string
	// maxFileSize is the maximum allowed file size in bytes (0 = no limit)
	maxFileSize int64
}

// NewService creates a new FileService
func NewService(basePath string, maxFileSize int64) *Service {
	return &Service{
		basePath:    basePath,
		maxFileSize: maxFileSize,
	}
}

// GetFileInfo returns information about a file
func (s *Service) GetFileInfo(path string) (*FileInfo, error) {
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		if os.IsPermission(err) {
			return nil, ErrPermissionDenied
		}
		return nil, err
	}

	if stat.IsDir() {
		return nil, ErrInvalidPath
	}

	ext := filepath.Ext(stat.Name())
	if ext != "" {
		ext = strings.ToLower(ext[1:]) // Remove leading dot and lowercase
	}

	return &FileInfo{
		Path:      path,
		Name:      stat.Name(),
		Extension: ext,
		Size:      stat.Size(),
		ModTime:   stat.ModTime(),
	}, nil
}

// GetFileContent returns a reader for the file content
func (s *Service) GetFileContent(path string) (io.ReadCloser, error) {
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		if os.IsPermission(err) {
			return nil, ErrPermissionDenied
		}
		return nil, err
	}

	return file, nil
}

// SaveFile saves content to a file
func (s *Service) SaveFile(path string, content io.Reader) error {
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ErrSaveFailed
	}

	// Create temporary file in the same directory
	tempFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return ErrSaveFailed
	}
	tempPath := tempFile.Name()
	defer func() {
		tempFile.Close()
		os.Remove(tempPath) // Clean up temp file on error
	}()

	// Copy content to temp file with size limit check
	var written int64
	if s.maxFileSize > 0 {
		written, err = io.CopyN(tempFile, content, s.maxFileSize+1)
		if written > s.maxFileSize {
			return ErrFileTooLarge
		}
		if err != nil && err != io.EOF {
			return ErrSaveFailed
		}
	} else {
		written, err = io.Copy(tempFile, content)
		if err != nil {
			return ErrSaveFailed
		}
	}

	// Close temp file before rename
	if err := tempFile.Close(); err != nil {
		return ErrSaveFailed
	}

	// Atomic rename
	if err := os.Rename(tempPath, fullPath); err != nil {
		return ErrSaveFailed
	}

	return nil
}

// resolvePath resolves and validates the file path
func (s *Service) resolvePath(path string) (string, error) {
	if path == "" {
		return "", ErrInvalidPath
	}

	// Normalize path: ensure it starts with "/" for consistency
	// This handles the difference between iPad (vol2/...) and desktop (/vol2/...)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Clean the path
	cleanPath := filepath.Clean(path)

	// If basePath is set, ensure the path is within it
	if s.basePath != "" {
		// If path is relative, join with basePath
		if !filepath.IsAbs(cleanPath) {
			cleanPath = filepath.Join(s.basePath, cleanPath)
		}

		// Ensure the resolved path is within basePath
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return "", ErrInvalidPath
		}

		absBase, err := filepath.Abs(s.basePath)
		if err != nil {
			return "", ErrInvalidPath
		}

		// Check for path traversal
		if !strings.HasPrefix(absPath, absBase) {
			return "", ErrInvalidPath
		}

		return absPath, nil
	}

	// If no basePath, just return the cleaned path
	return cleanPath, nil
}

// GetBasePath returns the base path for file operations
func (s *Service) GetBasePath() string {
	return s.basePath
}
