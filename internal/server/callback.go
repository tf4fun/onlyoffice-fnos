package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"onlyoffice-fnos/internal/config"
	jwtpkg "onlyoffice-fnos/internal/jwt"
)

// CallbackStatus represents the document status from OnlyOffice
type CallbackStatus int

const (
	// StatusEditing - document is being edited
	StatusEditing CallbackStatus = 1
	// StatusSaved - document is ready for saving
	StatusSaved CallbackStatus = 2
	// StatusSaveError - document saving error
	StatusSaveError CallbackStatus = 3
	// StatusClosed - document closed with no changes
	StatusClosed CallbackStatus = 4
	// StatusForceSave - document force save requested
	StatusForceSave CallbackStatus = 6
	// StatusForceSaveError - document force save error
	StatusForceSaveError CallbackStatus = 7
)

// CallbackAction represents an action in the callback
type CallbackAction struct {
	Type   int    `json:"type"`
	UserID string `json:"userid"`
}

// CallbackRequest represents the callback request from OnlyOffice Document Server
type CallbackRequest struct {
	Actions    []CallbackAction `json:"actions,omitempty"`
	Key        string           `json:"key"`
	Status     CallbackStatus   `json:"status"`
	Users      []string         `json:"users,omitempty"`
	URL        string           `json:"url,omitempty"`
	Token      string           `json:"token,omitempty"`
	Changesurl string           `json:"changesurl,omitempty"`
	History    json.RawMessage  `json:"history,omitempty"`
	Filetype   string           `json:"filetype,omitempty"`
}

// CallbackResponse represents the response to the callback
type CallbackResponse struct {
	Error int `json:"error"`
}

// handleCallback handles POST /callback
// This endpoint receives save notifications from OnlyOffice Document Server
func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Get file path from query parameter
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		log.Printf("Callback error: missing file path")
		s.respondJSON(w, http.StatusOK, &CallbackResponse{Error: 1})
		return
	}

	// Parse callback request
	var req CallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Callback error: failed to parse request: %v", err)
		s.respondJSON(w, http.StatusOK, &CallbackResponse{Error: 1})
		return
	}

	log.Printf("Callback received: path=%s, status=%d, key=%s", filePath, req.Status, req.Key)

	// Load settings to get JWT secret
	settings, err := s.settingsStore.Load()
	if err != nil && err != config.ErrConfigNotFound {
		log.Printf("Callback error: failed to load settings: %v", err)
		s.respondJSON(w, http.StatusOK, &CallbackResponse{Error: 1})
		return
	}

	// Verify JWT token if secret is configured
	if settings != nil && settings.DocumentServerSecret != "" {
		if req.Token == "" {
			log.Printf("Callback error: missing JWT token")
			s.respondJSON(w, http.StatusOK, &CallbackResponse{Error: 1})
			return
		}

		_, err := s.jwtManager.Verify(settings.DocumentServerSecret, req.Token)
		if err != nil {
			log.Printf("Callback error: invalid JWT token: %v", err)
			if err == jwtpkg.ErrExpiredToken {
				log.Printf("Callback error: token has expired")
			}
			s.respondJSON(w, http.StatusOK, &CallbackResponse{Error: 1})
			return
		}
	}

	// Handle different statuses
	switch req.Status {
	case StatusEditing:
		// Document is being edited, nothing to do
		log.Printf("Document %s is being edited", filePath)

	case StatusSaved, StatusForceSave:
		// Document is ready for saving
		if req.URL == "" {
			log.Printf("Callback error: missing document URL for save")
			s.respondJSON(w, http.StatusOK, &CallbackResponse{Error: 1})
			return
		}

		if err := s.saveDocument(filePath, req.URL); err != nil {
			log.Printf("Callback error: failed to save document: %v", err)
			s.respondJSON(w, http.StatusOK, &CallbackResponse{Error: 1})
			return
		}
		log.Printf("Document %s saved successfully", filePath)

	case StatusClosed:
		// Document closed with no changes
		log.Printf("Document %s closed with no changes", filePath)

	case StatusSaveError, StatusForceSaveError:
		// Save error occurred
		log.Printf("Document %s save error reported by Document Server", filePath)

	default:
		log.Printf("Unknown callback status %d for document %s", req.Status, filePath)
	}

	// Return success
	s.respondJSON(w, http.StatusOK, &CallbackResponse{Error: 0})
}

// saveDocument downloads the document from the given URL and saves it to the file path
func (s *Server) saveDocument(filePath, documentURL string) error {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Minute, // Allow longer timeout for large files
	}

	// Download the document
	resp, err := client.Get(documentURL)
	if err != nil {
		return fmt.Errorf("failed to download document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("document server returned status %d", resp.StatusCode)
	}

	// Save the document
	if err := s.fileService.SaveFile(filePath, resp.Body); err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}

	return nil
}

// SaveDocumentFromReader saves document content from a reader (for testing)
func (s *Server) SaveDocumentFromReader(filePath string, content io.Reader) error {
	return s.fileService.SaveFile(filePath, content)
}
