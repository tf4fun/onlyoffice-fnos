package server

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SettingsResponse represents the settings API response
type SettingsResponse struct {
	DocumentServerURL    string `json:"documentServerUrl"`
	DocumentServerSecret string `json:"documentServerSecret"`
	BaseURL              string `json:"baseUrl"`
}

// handleGetSettings handles GET /api/settings
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	if s.settings == nil {
		s.respondJSON(w, http.StatusOK, &SettingsResponse{})
		return
	}

	s.respondJSON(w, http.StatusOK, &SettingsResponse{
		DocumentServerURL:    s.settings.DocumentServerURL,
		DocumentServerSecret: s.settings.DocumentServerSecret,
		BaseURL:              s.settings.BaseURL,
	})
}

// handleGenerateKey handles POST /api/settings/generate-key
func (s *Server) handleGenerateKey(w http.ResponseWriter, r *http.Request) {
	newSecret := s.jwtManager.GenerateSecret()

	// For htmx requests, return an input element with the new secret
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<input type="text" id="secret" name="documentServerSecret" class="input-field" value="%s">`, newSecret)
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"secret": newSecret,
	})
}

// handleValidateConnection handles POST /api/settings/validate
func (s *Server) handleValidateConnection(w http.ResponseWriter, r *http.Request) {
	var serverURL string

	// Support form data
	if err := r.ParseForm(); err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid form data")
		return
	}
	serverURL = r.FormValue("documentServerUrl")

	if serverURL == "" && s.settings != nil {
		serverURL = s.settings.DocumentServerURL
	}

	if serverURL == "" {
		s.respondError(w, http.StatusBadRequest, "Document Server URL is required")
		return
	}

	// Normalize URL
	serverURL = strings.TrimSuffix(serverURL, "/")

	// Try to connect to the Document Server
	valid, err := s.validateDocumentServer(serverURL)

	// For htmx requests, return HTML status
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if valid {
			w.Write([]byte(`<div class="message success">连接成功！</div>`))
		} else {
			errMsg := "连接失败"
			if err != nil {
				errMsg = fmt.Sprintf("连接失败: %s", err.Error())
			}
			fmt.Fprintf(w, `<div class="message error">%s</div>`, errMsg)
		}
		return
	}

	if valid {
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"valid":   true,
			"message": "Connection successful",
		})
	} else {
		errMsg := "Connection failed"
		if err != nil {
			errMsg = err.Error()
		}
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"valid":   false,
			"message": errMsg,
		})
	}
}

// validateDocumentServer checks if the Document Server is accessible
func (s *Server) validateDocumentServer(serverURL string) (bool, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Try healthcheck endpoint first
	healthURL := serverURL + "/healthcheck"
	resp, err := client.Get(healthURL)
	if err != nil {
		// Try the web-apps endpoint as fallback
		webAppsURL := serverURL + "/web-apps/apps/api/documents/api.js"
		resp, err = client.Get(webAppsURL)
		if err != nil {
			return false, fmt.Errorf("cannot connect to server: %v", err)
		}
	}
	defer resp.Body.Close()

	// Read response body (limited to prevent memory issues)
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024*1024))

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, nil
	}

	return false, fmt.Errorf("server returned status %d", resp.StatusCode)
}
